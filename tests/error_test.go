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
	// opt.Debug = true

	impl := NewMemFileSystem()
	fssrv, err := fuse.NewServer(impl, "/tmp/memfs", &opt)
	if err != nil {
		panic(err)
	}

	// go writeFileWhenError("t1.txt", 2)
	go openFileWhenError("t2.txt", 5)
	// go lookupFileWhenError("t3.txt", 8)

	go func() {
		time.Sleep(time.Second * 25)
		os.Exit(0)
	}()

	fssrv.Serve()
}

func lookupFileWhenError(fileName string, sleepSeconds int) {
	mock_invalid_inode.Store(false)
	time.Sleep(time.Second * time.Duration(sleepSeconds))

	err := os.WriteFile("/tmp/memfs/"+fileName, []byte("Hello!"), 0644)
	if err != nil {
		panic(err)
	}

	mock_invalid_inode.Store(true)

	// 打开文件用于读写
	file, err := os.OpenFile("/tmp/memfs/"+fileName, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// 读取数据
	data := make([]byte, 20)
	n, err := file.Read(data)
	if err != nil && err != io.EOF {
		log.Fatal(err)
	}

	log.Printf("%s, pos=0, 读取内容: %s", fileName, data[:n])
}

func openFileWhenError(fileName string, sleepSeconds int) {
	mock_invalid_inode.Store(false)
	time.Sleep(time.Second * time.Duration(sleepSeconds))

	err1 := os.Mkdir("/tmp/memfs/d1", 0777)
	if err1 != nil {
		panic(err1)
	}

	log.Printf("\n\nWriteFile /tmp/memfs/d1/" + fileName + "\n")
	err := os.WriteFile("/tmp/memfs/d1/"+fileName, []byte("Hello!"), 0644)
	if err != nil {
		panic(err)
	}

	mock_invalid_inode.Store(true)

	for i := 0; i < 5; i++ {
		log.Printf("OpenFile /tmp/memfs/d1/%s,  %d", fileName, i)
		file, err := os.OpenFile("/tmp/memfs/d1/"+fileName, os.O_RDWR|os.O_CREATE, 0666)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		// 读取数据
		data := make([]byte, 20)
		n, err := file.Read(data)
		if err != nil && err != io.EOF {
			log.Fatal(err)
		}
		log.Printf("%s, pos=0, 读取内容: %s", fileName, data[:n])
	}

}

func writeFileWhenError(fileName string, sleepSeconds int) {
	mock_invalid_inode.Store(false)
	time.Sleep(time.Second * time.Duration(sleepSeconds))

	// 打开文件用于读写
	file, err := os.OpenFile("/tmp/memfs/"+fileName, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// 写入数据
	for i := 0; i < 10; i++ {
		if i > 0 && (i%3 == 0) {
			mock_invalid_inode.Store(true)
		}

		log.Printf("\n%s, write line %d\n", fileName, i)
		_, err = file.WriteString(getLineData(i))
		log.Printf("%s, write line return : %v", fileName, err)
		// if err != nil {
		// 	log.Fatal(err)
		// }
	}

	// 移动文件指针
	pos := int64(4096 * 3)
	log.Printf("%s Seek to: %d\n", fileName, pos)
	_, err = file.Seek(pos, 0)
	if err != nil {
		log.Fatal(err)
	}

	// 读取数据
	data := make([]byte, 20)
	n, err := file.Read(data)
	if err != nil && err != io.EOF {
		log.Fatal(err)
	}

	log.Printf("%s, 读取内容: %s", fileName, data[:n])
}

func getLineData(i int) string {
	return fmt.Sprintf("xxxx%-2d%4090d", i, 0)
}
