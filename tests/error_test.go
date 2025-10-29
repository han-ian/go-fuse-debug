package tests

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"testing"
	"time"

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
	go createAndReadFile(2)
	// go createAndReadFile(5)
	fssrv.Serve()
}

func createAndReadFile(sleepSeconds int) {

	mock_invalid_inode.Store(false)

	time.Sleep(time.Second * time.Duration(sleepSeconds))

	// 打开文件用于读写
	file, err := os.OpenFile("/tmp/memfs/data.txt", os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// 写入数据
	for i := 0; i < 20; i++ {
		if i > 0 && (i%5 == 0) {
			mock_invalid_inode.Store(true)
		}

		log.Printf("write line %d\n", i)
		_, err = file.WriteString(getLineData(i))
		if err != nil {
			log.Fatal(err)
		}
	}

	// 移动文件指针到开头
	_, err = file.Seek(4096*10, 0)
	if err != nil {
		log.Fatal(err)
	}

	// 读取数据
	data := make([]byte, 20)
	n, err := file.Read(data)
	if err != nil && err != io.EOF {
		log.Fatal(err)
	}

	log.Printf("读取内容: %s", data[:n])
}

func getLineData(i int) string {
	return fmt.Sprintf("%-4096d", i)
}
