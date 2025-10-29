package tests

import (
	"os/exec"
	"testing"

	"github.com/hanwen/go-fuse/v2/fuse"
)

func TestError2(t *testing.T) {
	exec.Command("umount", "/tmp/memfs").Run()

	var opt fuse.MountOptions
	impl := NewMemFileSystem()
	fssrv, err := fuse.NewServer(impl, "/tmp/memfs", &opt)
	if err != nil {
		panic(err)
	}
	fssrv.Serve()

}
