package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/Stezok/huffman/internal/archive"
)

func main() {
	arch := archive.NewArchiver()
	file, err := os.Create("output")
	if err != nil {
		fmt.Println(err)
	}

	arch.Compress("test.exe", file)
	res, _ := os.Create("result.exe")
	file.Seek(0, io.SeekStart)
	err = arch.Decompress(file, res)
	log.Print(err)
	file.Close()
}
