package tests

import (
	"log"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fuse"
)

var mock_invalid_inode atomic.Bool
var op_nums atomic.Int32
var open_nums atomic.Int32

type memFileSystem struct {
	fuse.RawFileSystem
	nodes  map[uint64]*memNode // Map node ID to nodes
	nextID uint64              // Next available node ID
}

type memNode struct {
	id       uint64
	name     string
	data     []byte
	isDir    bool
	children map[string]uint64 // Map of child names to node IDs
	mtime    time.Time
	mode     uint32
}

// NewMemFileSystem creates a new in-memory filesystem
func NewMemFileSystem() *memFileSystem {
	log.Printf("NewMemFileSystem()")
	fs := &memFileSystem{
		nodes:  make(map[uint64]*memNode),
		nextID: 2, // Start from 2, reserving 1 for root
	}

	// Create root directory
	root := &memNode{
		id:       1,
		name:     "/",
		isDir:    true,
		children: make(map[string]uint64),
		mtime:    time.Now(),
		mode:     syscall.S_IFDIR | 0755,
	}
	fs.nodes[root.id] = root

	return fs
}

// String returns a string representation of the filesystem
func (m *memFileSystem) String() string {
	log.Printf("String()")
	return "memFileSystem"
}

// SetDebug enables or disables debug mode
func (m *memFileSystem) SetDebug(debug bool) {
	log.Printf("SetDebug(debug=%v)", debug)
	// Implementation can be added if needed
}

// Init initializes the filesystem
func (m *memFileSystem) Init(s *fuse.Server) {
	log.Printf("Init()")
	// Initialization logic if needed
}

// Lookup finds a child node by name
func (m *memFileSystem) Lookup(cancel <-chan struct{}, header *fuse.InHeader, name string, out *fuse.EntryOut) (status fuse.Status) {

	log.Printf("Lookup(header.NodeId=%d, name=%s)", header.NodeId, name)

	op_nums.Add(1)
	// if ops := op_nums.Load(); ops%2 == 0 && header.NodeId != 1 || mock_invalid_inode.Load() {
	// 	mock_invalid_inode.Store(false)

	// 	log.Printf("Lookup(input.NodeId=%d), mock invalid inode, return ERROR\n", header.NodeId)
	// 	return fuse.Status(syscall.ESTALE)
	// }

	node, ok := m.nodes[header.NodeId]
	if !ok {
		log.Printf("Lookup: node %d not found", header.NodeId)
		return fuse.Status(syscall.ENOENT)
	}

	if !node.isDir {
		log.Printf("Lookup: node %d is not a directory", header.NodeId)
		return fuse.Status(syscall.ENOTDIR)
	}

	childID, ok := node.children[name]
	if !ok {
		log.Printf("Lookup: child %s not found in node %d", name, header.NodeId)
		return fuse.Status(syscall.ENOENT)
	}

	child, ok := m.nodes[childID]
	if !ok {
		log.Printf("Lookup: child node %d not found", childID)
		return fuse.Status(syscall.ENOENT)
	}

	out.NodeId = child.id
	out.Generation = 0
	setAttr(&out.Attr, child)

	log.Printf("Lookup: found node %d for name %s", child.id, name)
	return fuse.OK
}

// GetAttr returns file attributes
func (m *memFileSystem) GetAttr(cancel <-chan struct{}, input *fuse.GetAttrIn, out *fuse.AttrOut) (code fuse.Status) {
	log.Printf("GetAttr(input.NodeId=%d)", input.NodeId)

	node, ok := m.nodes[input.NodeId]
	if !ok {
		log.Printf("GetAttr: node %d not found", input.NodeId)
		return fuse.Status(syscall.ENOENT)
	}

	setAttr(&out.Attr, node)
	out.AttrValid = 10
	out.AttrValidNsec = 1000000000

	log.Printf("GetAttr: returning attributes for node %d", input.NodeId)
	return fuse.OK
}

// SetAttr sets file attributes
func (m *memFileSystem) SetAttr(cancel <-chan struct{}, input *fuse.SetAttrIn, out *fuse.AttrOut) (code fuse.Status) {
	log.Printf("SetAttr(input.NodeId=%d, input.Valid=%d)", input.NodeId, input.Valid)

	node, ok := m.nodes[input.NodeId]
	if !ok {
		log.Printf("SetAttr: node %d not found", input.NodeId)
		return fuse.Status(syscall.ENOENT)
	}

	if input.Valid&fuse.FATTR_MODE != 0 {
		log.Printf("SetAttr: setting mode from %o to %o", node.mode, (node.mode&^07777)|(input.Mode&07777))
		node.mode = (node.mode &^ 07777) | (input.Mode & 07777)
	}

	if input.Valid&fuse.FATTR_SIZE != 0 {
		if node.isDir {
			log.Printf("SetAttr: attempting to truncate directory %d", input.NodeId)
			return fuse.Status(syscall.EISDIR)
		}
		log.Printf("SetAttr: resizing node %d from %d to %d bytes", input.NodeId, len(node.data), input.Size)
		if input.Size > uint64(len(node.data)) {
			newData := make([]byte, input.Size)
			copy(newData, node.data)
			node.data = newData
		} else {
			node.data = node.data[:input.Size]
		}
	}

	if input.Valid&fuse.FATTR_MTIME != 0 {
		log.Printf("SetAttr: setting mtime for node %d", input.NodeId)
		node.mtime = time.Unix(int64(input.Mtime), int64(input.Mtimensec))
	} else {
		log.Printf("SetAttr: updating mtime for node %d", input.NodeId)
		node.mtime = time.Now()
	}

	setAttr(&out.Attr, node)
	log.Printf("SetAttr: completed for node %d", input.NodeId)
	return fuse.OK
}

// Mkdir creates a directory
func (m *memFileSystem) Mkdir(cancel <-chan struct{}, input *fuse.MkdirIn, name string, out *fuse.EntryOut) (code fuse.Status) {
	log.Printf("Mkdir(input.NodeId=%d, name=%s, mode=%o)", input.NodeId, name, input.Mode)

	op_nums.Add(1)
	if ops := op_nums.Load(); ops%2 == 0 && input.NodeId != 1 {
		log.Printf("Mkdir(input.NodeId=%d), mock invalid inode, return ERROR\n", input.NodeId)
		return fuse.Status(syscall.ESTALE)
	}

	parent, ok := m.nodes[input.NodeId]
	if !ok {
		log.Printf("Mkdir: parent node %d not found", input.NodeId)
		return fuse.Status(syscall.ENOENT)
	}

	if !parent.isDir {
		log.Printf("Mkdir: parent node %d is not a directory", input.NodeId)
		return fuse.Status(syscall.ENOTDIR)
	}

	if _, exists := parent.children[name]; exists {
		log.Printf("Mkdir: directory %s already exists in node %d", name, input.NodeId)
		return fuse.Status(syscall.EEXIST)
	}

	// Create new directory node
	childID := m.nextID
	m.nextID++

	log.Printf("Mkdir: creating new directory node %d with name %s", childID, name)

	child := &memNode{
		id:       childID,
		name:     name,
		isDir:    true,
		children: make(map[string]uint64),
		mtime:    time.Now(),
		mode:     syscall.S_IFDIR | (input.Mode & 07777),
	}

	m.nodes[childID] = child
	parent.children[name] = childID

	out.NodeId = childID
	out.Generation = 0
	setAttr(&out.Attr, child)

	log.Printf("Mkdir: successfully created directory %s with node id %d", name, childID)
	return fuse.OK
}

// Create creates a file
func (m *memFileSystem) Create(cancel <-chan struct{}, input *fuse.CreateIn, name string, out *fuse.CreateOut) (code fuse.Status) {
	log.Printf("Create(input.NodeId=%d, name=%s, mode=%o)", input.NodeId, name, input.Mode)

	op_nums.Add(1)
	if ops := op_nums.Load(); ops%2 == 0 && input.NodeId != 1 {
		log.Printf("Create(input.NodeId=%d), mock invalid inode, return ERROR\n", input.NodeId)
		return fuse.Status(syscall.ESTALE)
	}

	parent, ok := m.nodes[input.NodeId]
	if !ok {
		log.Printf("Create: parent node %d not found", input.NodeId)
		return fuse.Status(syscall.ENOENT)
	}

	if !parent.isDir {
		log.Printf("Create: parent node %d is not a directory", input.NodeId)
		return fuse.Status(syscall.ENOTDIR)
	}

	if _, exists := parent.children[name]; exists {
		// If file exists, we should handle according to flags
		// For simplicity, we'll just return success
		childID := parent.children[name]
		child := m.nodes[childID]
		if child != nil {
			log.Printf("Create: file %s already exists with node id %d", name, childID)
			out.NodeId = childID
			out.Generation = 0
			setAttr(&out.Attr, child)
			out.OpenOut.Fh = 0
			return fuse.OK
		}
	}

	// Create new file node
	childID := m.nextID
	m.nextID++

	log.Printf("Create: creating new file node %d with name %s", childID, name)

	child := &memNode{
		id:    childID,
		name:  name,
		data:  make([]byte, 0),
		mtime: time.Now(),
		mode:  syscall.S_IFREG | (input.Mode & 07777),
	}

	m.nodes[childID] = child
	parent.children[name] = childID

	out.NodeId = childID
	out.Generation = 0
	setAttr(&out.Attr, child)
	out.OpenOut.Fh = 0

	log.Printf("Create: successfully created file %s with node id %d", name, childID)
	return fuse.OK
}

// Open opens a file
func (m *memFileSystem) Open(cancel <-chan struct{}, input *fuse.OpenIn, out *fuse.OpenOut) (status fuse.Status) {
	log.Printf("Open(input.NodeId=%d, input.Flags=%d)", input.NodeId, input.Flags)

	open_nums.Add(1)
	if ops := open_nums.Load(); ops%2 == 0 && input.NodeId != 1 {
		log.Printf("Open(input.NodeId=%d), mock invalid inode, return ERROR\n", input.NodeId)
		// return fuse.Status(syscall.ESTALE)
		return fuse.Status(syscall.ESTALE)
	}

	_, ok := m.nodes[input.NodeId]
	if !ok {
		log.Printf("Open: node %d not found", input.NodeId)
		return fuse.Status(syscall.ENOENT)
	}

	out.Fh = 0
	log.Printf("Open: successfully opened node %d", input.NodeId)
	return fuse.OK
}

// Read reads data from a file
func (m *memFileSystem) Read(cancel <-chan struct{}, input *fuse.ReadIn, buf []byte) (fuse.ReadResult, fuse.Status) {
	log.Printf("Read(input.NodeId=%d, input.Offset=%d, len(buf)=%d)", input.NodeId, input.Offset, len(buf))

	node, ok := m.nodes[input.NodeId]
	if !ok {
		log.Printf("Read: node %d not found", input.NodeId)
		return nil, fuse.Status(syscall.ENOENT)
	}

	if node.isDir {
		log.Printf("Read: attempting to read from directory node %d", input.NodeId)
		return nil, fuse.Status(syscall.EISDIR)
	}

	if int(input.Offset) >= len(node.data) {
		log.Printf("Read: offset %d beyond file size %d", input.Offset, len(node.data))
		return fuse.ReadResultData([]byte{}), fuse.OK
	}

	end := int(input.Offset) + len(buf)
	if end > len(node.data) {
		end = len(node.data)
	}

	data := node.data[input.Offset:end]
	log.Printf("Read: returning %d bytes from offset %d", len(data), input.Offset)
	return fuse.ReadResultData(data), fuse.OK
}

// Write writes data to a file
func (m *memFileSystem) Write(cancel <-chan struct{}, input *fuse.WriteIn, data []byte) (written uint32, code fuse.Status) {
	log.Printf("Write(input.NodeId=%d, input.Offset=%d, len(data)=%d)", input.NodeId, input.Offset, len(data))

	// if mock_invalid_inode.Load() {
	// 	mock_invalid_inode.Store(false)

	// 	log.Printf("Write: mock invalid inode, returning Error for node %d", input.NodeId)
	// 	// return 0, fuse.Status(syscall.EBADFD)
	// 	return 0, fuse.Status(syscall.ESTALE)
	// }

	node, ok := m.nodes[input.NodeId]
	if !ok {
		log.Printf("Write: node %d not found", input.NodeId)
		return 0, fuse.Status(syscall.ENOENT)
	}

	if node.isDir {
		log.Printf("Write: attempting to write to directory node %d", input.NodeId)
		return 0, fuse.Status(syscall.EISDIR)
	}

	end := int(input.Offset) + len(data)
	if end > len(node.data) {
		log.Printf("Write: extending file from %d to %d bytes", len(node.data), end)
		newData := make([]byte, end)
		copy(newData, node.data)
		node.data = newData
	}

	copy(node.data[input.Offset:], data)
	node.mtime = time.Now()

	log.Printf("Write: wrote %d bytes to node %d", len(data), input.NodeId)
	return uint32(len(data)), fuse.OK
}

// Unlink removes a file
func (m *memFileSystem) Unlink(cancel <-chan struct{}, header *fuse.InHeader, name string) (code fuse.Status) {
	log.Printf("Unlink(header.NodeId=%d, name=%s)", header.NodeId, name)

	parent, ok := m.nodes[header.NodeId]
	if !ok {
		log.Printf("Unlink: parent node %d not found", header.NodeId)
		return fuse.Status(syscall.ENOENT)
	}

	if !parent.isDir {
		log.Printf("Unlink: parent node %d is not a directory", header.NodeId)
		return fuse.Status(syscall.ENOTDIR)
	}

	childID, exists := parent.children[name]
	if !exists {
		log.Printf("Unlink: file %s not found in node %d", name, header.NodeId)
		return fuse.Status(syscall.ENOENT)
	}

	child, ok := m.nodes[childID]
	if !ok {
		log.Printf("Unlink: child node %d not found", childID)
		return fuse.Status(syscall.ENOENT)
	}

	if child.isDir {
		log.Printf("Unlink: attempting to unlink directory %s (node %d)", name, childID)
		return fuse.Status(syscall.EISDIR)
	}

	delete(parent.children, name)
	delete(m.nodes, childID)

	log.Printf("Unlink: successfully removed file %s (node %d)", name, childID)
	return fuse.OK
}

// Rmdir removes a directory
func (m *memFileSystem) Rmdir(cancel <-chan struct{}, header *fuse.InHeader, name string) (code fuse.Status) {
	log.Printf("Rmdir(header.NodeId=%d, name=%s)", header.NodeId, name)

	parent, ok := m.nodes[header.NodeId]
	if !ok {
		log.Printf("Rmdir: parent node %d not found", header.NodeId)
		return fuse.Status(syscall.ENOENT)
	}

	if !parent.isDir {
		log.Printf("Rmdir: parent node %d is not a directory", header.NodeId)
		return fuse.Status(syscall.ENOTDIR)
	}

	childID, exists := parent.children[name]
	if !exists {
		log.Printf("Rmdir: directory %s not found in node %d", name, header.NodeId)
		return fuse.Status(syscall.ENOENT)
	}

	child, ok := m.nodes[childID]
	if !ok {
		log.Printf("Rmdir: child node %d not found", childID)
		return fuse.Status(syscall.ENOENT)
	}

	if !child.isDir {
		log.Printf("Rmdir: %s (node %d) is not a directory", name, childID)
		return fuse.Status(syscall.ENOTDIR)
	}

	// Check if directory is empty
	if len(child.children) > 0 {
		log.Printf("Rmdir: directory %s (node %d) is not empty", name, childID)
		return fuse.Status(syscall.ENOTEMPTY)
	}

	delete(parent.children, name)
	delete(m.nodes, childID)

	log.Printf("Rmdir: successfully removed directory %s (node %d)", name, childID)
	return fuse.OK
}

// Rename renames a file or directory
func (m *memFileSystem) Rename(cancel <-chan struct{}, input *fuse.RenameIn, oldName string, newName string) (code fuse.Status) {
	log.Printf("Rename(input.NodeId=%d, input.Newdir=%d, oldName=%s, newName=%s)", input.NodeId, input.Newdir, oldName, newName)

	oldParent, ok := m.nodes[input.NodeId]
	if !ok {
		log.Printf("Rename: old parent node %d not found", input.NodeId)
		return fuse.Status(syscall.ENOENT)
	}

	if !oldParent.isDir {
		log.Printf("Rename: old parent node %d is not a directory", input.NodeId)
		return fuse.Status(syscall.ENOTDIR)
	}

	newParent, ok := m.nodes[input.Newdir]
	if !ok {
		log.Printf("Rename: new parent node %d not found", input.Newdir)
		return fuse.Status(syscall.ENOENT)
	}

	if !newParent.isDir {
		log.Printf("Rename: new parent node %d is not a directory", input.Newdir)
		return fuse.Status(syscall.ENOTDIR)
	}

	childID, exists := oldParent.children[oldName]
	if !exists {
		log.Printf("Rename: source %s not found in node %d", oldName, input.NodeId)
		return fuse.Status(syscall.ENOENT)
	}

	// Remove from old location
	delete(oldParent.children, oldName)

	// Add to new location
	newParent.children[newName] = childID

	// Update the node's name
	if child, ok := m.nodes[childID]; ok {
		log.Printf("Rename: renaming node %d from %s to %s", childID, oldName, newName)
		child.name = newName
	}

	log.Printf("Rename: successfully renamed %s to %s", oldName, newName)
	return fuse.OK
}

// ReadDir reads directory entries
func (m *memFileSystem) ReadDir(cancel <-chan struct{}, input *fuse.ReadIn, out *fuse.DirEntryList) fuse.Status {
	log.Printf("ReadDir(input.NodeId=%d)", input.NodeId)

	op_nums.Add(1)
	if ops := op_nums.Load(); ops%2 == 0 && input.NodeId != 1 {
		log.Printf("ReadDir(input.NodeId=%d), mock invalid inode, return ERROR\n", input.NodeId)
		return fuse.Status(syscall.ESTALE)
	}

	node, ok := m.nodes[input.NodeId]
	if !ok {
		log.Printf("ReadDir: node %d not found", input.NodeId)
		return fuse.Status(syscall.ENOENT)
	}

	if !node.isDir {
		log.Printf("ReadDir: node %d is not a directory", input.NodeId)
		return fuse.Status(syscall.ENOTDIR)
	}

	count := 0
	for name, childID := range node.children {
		child, ok := m.nodes[childID]
		if !ok {
			continue
		}

		mode := uint32(syscall.S_IFREG)
		if child.isDir {
			mode = syscall.S_IFDIR
		}

		e := out.AddDirEntry(fuse.DirEntry{
			Name: name,
			Mode: mode,
			Ino:  childID,
		})

		count++
		if !e {
			log.Printf("ReadDir: buffer full after %d entries", count)
			break
		}
	}

	log.Printf("ReadDir: returning %d entries for node %d", count, input.NodeId)
	return fuse.OK
}

// StatFs returns filesystem statistics
func (m *memFileSystem) StatFs(cancel <-chan struct{}, input *fuse.InHeader, out *fuse.StatfsOut) (code fuse.Status) {
	log.Printf("StatFs(input.NodeId=%d)", input.NodeId)

	out.Blocks = 1000000
	out.Bfree = 1000000
	out.Bavail = 1000000
	out.Files = 1000000
	out.Ffree = 1000000
	out.Bsize = 4096
	out.NameLen = 255
	out.Frsize = 4096

	log.Printf("StatFs: returning filesystem statistics")
	return fuse.OK
}

// Forget is called when the kernel discards entries from its dentry cache
func (m *memFileSystem) Forget(nodeid, nlookup uint64) {
	log.Printf("Forget(nodeid=%d, nlookup=%d)", nodeid, nlookup)
	// In a real implementation, we might want to track lookup counts
	// For this simple implementation, we do nothing
}

// Release releases a file handle
func (m *memFileSystem) Release(cancel <-chan struct{}, input *fuse.ReleaseIn) {
	log.Printf("Release(input.NodeId=%d, input.Fh=%d)", input.NodeId, input.Fh)
	// For this simple implementation, we do nothing
}

// ReleaseDir releases a directory handle
func (m *memFileSystem) ReleaseDir(input *fuse.ReleaseIn) {
	log.Printf("ReleaseDir(input.NodeId=%d, input.Fh=%d)", input.NodeId, input.Fh)
	// For this simple implementation, we do nothing
}

// Mknod creates a special file node
func (m *memFileSystem) Mknod(cancel <-chan struct{}, input *fuse.MknodIn, name string, out *fuse.EntryOut) (code fuse.Status) {
	log.Printf("Mknod(input.NodeId=%d, name=%s) - not implemented", input.NodeId, name)
	return fuse.ENOSYS
}

// Link creates a hard link
func (m *memFileSystem) Link(cancel <-chan struct{}, input *fuse.LinkIn, filename string, out *fuse.EntryOut) (code fuse.Status) {
	log.Printf("Link(input.NodeId=%d, filename=%s) - not implemented", input.NodeId, filename)
	return fuse.ENOSYS
}

// Symlink creates a symbolic link
func (m *memFileSystem) Symlink(cancel <-chan struct{}, header *fuse.InHeader, pointedTo string, linkName string, out *fuse.EntryOut) (code fuse.Status) {
	log.Printf("Symlink(header.NodeId=%d, pointedTo=%s, linkName=%s) - not implemented", header.NodeId, pointedTo, linkName)
	return fuse.ENOSYS
}

// Readlink reads the target of a symbolic link
func (m *memFileSystem) Readlink(cancel <-chan struct{}, header *fuse.InHeader) (out []byte, code fuse.Status) {
	log.Printf("Readlink(header.NodeId=%d) - not implemented", header.NodeId)
	return nil, fuse.ENOSYS
}

// Access checks access permissions for a node
func (m *memFileSystem) Access(cancel <-chan struct{}, input *fuse.AccessIn) (code fuse.Status) {
	log.Printf("Access(input.NodeId=%d, input.Mask=%d) - not implemented", input.NodeId, input.Mask)
	return fuse.ENOSYS
}

// GetXAttr gets extended attribute value
func (m *memFileSystem) GetXAttr(cancel <-chan struct{}, header *fuse.InHeader, attr string, dest []byte) (sz uint32, code fuse.Status) {
	log.Printf("GetXAttr(header.NodeId=%d, attr=%s) - not implemented", header.NodeId, attr)
	return 0, fuse.ENOSYS
}

// ListXAttr lists extended attributes
func (m *memFileSystem) ListXAttr(cancel <-chan struct{}, header *fuse.InHeader, dest []byte) (uint32, fuse.Status) {
	log.Printf("ListXAttr(header.NodeId=%d) - not implemented", header.NodeId)
	return 0, fuse.ENOSYS
}

// SetXAttr sets extended attribute value
func (m *memFileSystem) SetXAttr(cancel <-chan struct{}, input *fuse.SetXAttrIn, attr string, data []byte) fuse.Status {
	log.Printf("SetXAttr(input.NodeId=%d, attr=%s) - not implemented", input.NodeId, attr)
	return fuse.ENOSYS
}

// RemoveXAttr removes an extended attribute
func (m *memFileSystem) RemoveXAttr(cancel <-chan struct{}, header *fuse.InHeader, attr string) (code fuse.Status) {
	log.Printf("RemoveXAttr(header.NodeId=%d, attr=%s) - not implemented", header.NodeId, attr)
	return fuse.ENOSYS
}

// Lseek performs an lseek operation
func (m *memFileSystem) Lseek(cancel <-chan struct{}, in *fuse.LseekIn, out *fuse.LseekOut) fuse.Status {
	log.Printf("Lseek(in.NodeId=%d) - not implemented", in.NodeId)
	return fuse.ENOSYS
}

// GetLk gets file lock information
func (m *memFileSystem) GetLk(cancel <-chan struct{}, input *fuse.LkIn, out *fuse.LkOut) (code fuse.Status) {
	log.Printf("GetLk(input.NodeId=%d) - not implemented", input.NodeId)
	return fuse.ENOSYS
}

// SetLk sets a file lock
func (m *memFileSystem) SetLk(cancel <-chan struct{}, input *fuse.LkIn) (code fuse.Status) {
	log.Printf("SetLk(input.NodeId=%d) - not implemented", input.NodeId)
	return fuse.ENOSYS
}

// SetLkw sets a file lock and waits
func (m *memFileSystem) SetLkw(cancel <-chan struct{}, input *fuse.LkIn) (code fuse.Status) {
	log.Printf("SetLkw(input.NodeId=%d) - not implemented", input.NodeId)
	return fuse.ENOSYS
}

// CopyFileRange copies a range of data between files
func (m *memFileSystem) CopyFileRange(cancel <-chan struct{}, input *fuse.CopyFileRangeIn) (written uint32, code fuse.Status) {
	log.Printf("CopyFileRange(input.NodeId=%d) - not implemented", input.NodeId)
	return 0, fuse.ENOSYS
}

// Flush flushes file data
func (m *memFileSystem) Flush(cancel <-chan struct{}, input *fuse.FlushIn) fuse.Status {
	log.Printf("Flush(input.NodeId=%d, input.Fh=%d) - not implemented", input.NodeId, input.Fh)
	return fuse.ENOSYS
}

// Fsync synchronizes file data
func (m *memFileSystem) Fsync(cancel <-chan struct{}, input *fuse.FsyncIn) (code fuse.Status) {
	log.Printf("Fsync(input.NodeId=%d, input.Fh=%d) - not implemented", input.NodeId, input.Fh)
	return fuse.ENOSYS
}

// Fallocate preallocates file space
func (m *memFileSystem) Fallocate(cancel <-chan struct{}, input *fuse.FallocateIn) (code fuse.Status) {
	log.Printf("Fallocate(input.NodeId=%d, input.Fh=%d) - not implemented", input.NodeId, input.Fh)
	return fuse.ENOSYS
}

// OpenDir opens a directory
func (m *memFileSystem) OpenDir(cancel <-chan struct{}, input *fuse.OpenIn, out *fuse.OpenOut) (status fuse.Status) {
	log.Printf("OpenDir(input.NodeId=%d, input.Flags=%d)", input.NodeId, input.Flags)

	op_nums.Add(1)
	if ops := op_nums.Load(); ops%2 == 0 && input.NodeId != 1 {
		log.Printf("OpenDir(input.NodeId=%d), mock invalid inode, return ERROR\n", input.NodeId)
		return fuse.Status(syscall.ESTALE)
	}

	_, ok := m.nodes[input.NodeId]
	if !ok {
		log.Printf("OpenDir: node %d not found", input.NodeId)
		return fuse.Status(syscall.ENOENT)
	}

	out.Fh = 0
	log.Printf("OpenDir: successfully opened directory node %d", input.NodeId)
	return fuse.OK
}

// ReadDirPlus reads directory entries with full attributes
func (m *memFileSystem) ReadDirPlus(cancel <-chan struct{}, input *fuse.ReadIn, out *fuse.DirEntryList) fuse.Status {
	log.Printf("ReadDirPlus(input.NodeId=%d) - use ReadDir to replace it", input.NodeId)

	return m.ReadDir(cancel, input, out)
	// return fuse.ENOSYS
}

// FsyncDir synchronizes directory data
func (m *memFileSystem) FsyncDir(cancel <-chan struct{}, input *fuse.FsyncIn) (code fuse.Status) {
	log.Printf("FsyncDir(input.NodeId=%d, input.Fh=%d) - not implemented", input.NodeId, input.Fh)
	return fuse.ENOSYS
}

// Ioctl performs an ioctl operation
func (m *memFileSystem) Ioctl(cancel <-chan struct{}, in *fuse.IoctlIn, out *fuse.IoctlOut, bufIn, bufOut []byte) fuse.Status {
	log.Printf("Ioctl(in.NodeId=%d) - not implemented", in.NodeId)
	return fuse.ENOSYS
}

// Helper function to set file attributes
func setAttr(attr *fuse.Attr, node *memNode) {
	attr.Ino = node.id
	attr.Mode = node.mode
	attr.Nlink = 1

	if node.isDir {
		attr.Size = 4096
	} else {
		attr.Size = uint64(len(node.data))
	}

	attr.Mtime = uint64(node.mtime.Unix())
	attr.Mtimensec = uint32(node.mtime.Nanosecond())
	attr.Atime = uint64(node.mtime.Unix())
	attr.Atimensec = uint32(node.mtime.Nanosecond())
	attr.Ctime = uint64(node.mtime.Unix())
	attr.Ctimensec = uint32(node.mtime.Nanosecond())
	attr.Blksize = 4096
	attr.Blocks = (attr.Size + 511) / 512
}
