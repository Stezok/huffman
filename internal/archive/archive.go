package archive

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
)

const BUFFER_SIZE = 0x20000
const ENCODED_LIM = 0x200

type Logger interface {
	Print(...interface{})
}

type Node struct {
	Weight uint32
	Data   byte
	Left   *Node
	Right  *Node
}

type Archiver struct {
	logger Logger
}

func (arch *Archiver) countByteMeet(r io.Reader) ([256]uint32, error) {
	var byteMeetCount [256]uint32

	outBuffer := bytes.NewBuffer([]byte{})

	readBuffer := make([]byte, BUFFER_SIZE)

	var err error
	var read int
	for err != io.EOF {
		read, err = r.Read(readBuffer)
		if err != nil && err != io.EOF {
			arch.logger.Print(err)
			return byteMeetCount, err
		}

		outBuffer.Write(readBuffer[:read])

		for i := 0; i < read; i++ {
			byteMeetCount[int(readBuffer[i])]++
		}
	}
	return byteMeetCount, nil
}

func (arch *Archiver) fillMap(node *Node, code string, m map[byte]string) {
	if node.Left == node.Right {
		m[node.Data] = code
		return
	}

	arch.fillMap(node.Left, code+"1", m)
	arch.fillMap(node.Right, code+"0", m)
}

func (arch *Archiver) buildMap(byteMeetCount [256]uint32) map[byte]string {
	nodePool := []*Node{}
	for i := 0; i < 256; i++ {
		if byteMeetCount[i] != 0 {
			nodePool = append(nodePool, &Node{
				Weight: byteMeetCount[i],
				Data:   byte(i),
				Left:   nil,
				Right:  nil,
			})
		}
	}

	times := len(nodePool) - 1
	for ; times != 0; times-- {
		ind1, ind2 := 0, 1
		if nodePool[ind1].Weight > nodePool[ind2].Weight {
			ind1, ind2 = ind2, ind1
		}

		for i := 2; i < len(nodePool); i++ {
			if nodePool[ind1].Weight >= nodePool[i].Weight {
				ind1, ind2 = i, ind1
			} else if nodePool[ind2].Weight > nodePool[i].Weight {
				ind2 = i
			}
		}

		if ind1 > ind2 {
			ind1, ind2 = ind2, ind1
		}
		newNode := &Node{
			Weight: nodePool[ind1].Weight + nodePool[ind2].Weight,
			Left:   nodePool[ind1],
			Right:  nodePool[ind2],
		}

		newNodePool := nodePool[:0]
		for i := 0; i < len(nodePool); i++ {
			if i == ind1 || i == ind2 {
				continue
			}
			newNodePool = append(newNodePool, nodePool[i])
		}
		newNodePool = append(newNodePool, newNode)

		for i := len(newNodePool); i < len(nodePool); i++ {
			nodePool[i] = nil
		}
	}

	m := make(map[byte]string)
	arch.fillMap(nodePool[0], "", m)

	return m
}

type bitString string

func (bs bitString) AsByteSlice() (result []byte, extra string) {
	var bytes []byte
	for len(bs) >= 8 {
		stringByte := bs[:8]
		var byteValue byte
		for len(stringByte) != 0 {
			byteValue = byteValue<<1 + stringByte[len(stringByte)-1]
			stringByte = stringByte[:len(stringByte)-1]
		}
		bytes = append(bytes, byteValue)
	}
	return bytes, string(bs)
}

func (arhc *Archiver) writeEncoded(encoded *string, w io.Writer) error {
	var bytes []byte
	bytes, *encoded = bitString(*encoded).AsByteSlice()
	_, err := w.Write(bytes)
	return err
}

func (arch *Archiver) archiveFile(r io.Reader, byteMeetCount [256]uint32, w io.Writer) error {
	codeTable := arch.buildMap(byteMeetCount)

	var lastByteLen byte
	for byteVal, count := range byteMeetCount {
		add := byte(count) * byte(len(codeTable[byte(byteVal)]))
		lastByteLen = (lastByteLen + add) % 8
	}

	var err error
	for _, count := range byteMeetCount {
		block := make([]byte, 4)
		binary.BigEndian.PutUint32(block[0:4], count)
		_, err = w.Write(block)
		if err != nil {
			return err
		}
	}

	_, err = w.Write([]byte{lastByteLen})
	if err != nil {
		return err
	}

	var read int
	var encoded string
	readBuffer := make([]byte, BUFFER_SIZE)

	for err != io.EOF {
		read, err = r.Read(readBuffer)
		if err != nil && err != io.EOF {
			arch.logger.Print(err)
			return err
		}

		for i := 0; i < read; i++ {
			encoded += codeTable[readBuffer[i]]
			err = arch.writeEncoded(&encoded, w)
			if err != nil {
				return err
			}
		}
	}

	err = arch.writeEncoded(&encoded, w)
	if err != nil {
		return err
	}

	if len(encoded) != 0 {
		for len(encoded) != 8 {
			encoded += "0"
		}
		err = arch.writeEncoded(&encoded, w)
		if err != nil {
			return err
		}
	}

	return nil
}

func (arch *Archiver) Compress(filename string, output io.Writer) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	byteMeetCount, err := arch.countByteMeet(file)
	if err != nil {
		return err
	}

	file.Seek(0, io.SeekStart)
	arch.archiveFile(file, byteMeetCount, output)

	return nil
}

func (arhc *Archiver) Decompress(r io.Reader, w io.WriteCloser) error {
	return nil
}

func NewArchiver(logger Logger) *Archiver {
	return &Archiver{
		logger: logger,
	}
}
