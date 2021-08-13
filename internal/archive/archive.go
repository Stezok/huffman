package archive

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
)

const (
	BLOCK_SIZE = 0x10000
)

var (
	EmptyFileError = errors.New("File is empty or unreadable")
)

type Node struct {
	Weight uint32
	Data   byte
	Left   *Node
	Right  *Node
}

func toBitsAsString(b byte) string {
	var result string
	for i := 7; i >= 0; i-- {
		result = fmt.Sprint(result, b>>i&1)
	}
	return result
}

type bitString string

func (bs bitString) AsByteSlice() (bytes []byte, extra string) {
	for len(bs) >= 8 {
		stringByte := bs[:8]
		bs = bs[8:]

		var byteValue byte
		for i := 0; i < 8; i++ {
			byteValue = byteValue<<1 + (stringByte[i] - '0')
		}
		bytes = append(bytes, byteValue)
	}
	extra = string(bs)
	return
}

type Archiver struct {
}

func (arch *Archiver) countBytes(r io.Reader) (byteCount [256]uint32, err error) {
	buf := make([]byte, BLOCK_SIZE)

	var read int
	for err != io.EOF {
		read, err = r.Read(buf)
		if err != nil && err != io.EOF {
			return
		}

		for i := 0; i < read; i++ {
			byteCount[int(buf[i])]++
		}
	}
	return byteCount, nil
}

func (arch *Archiver) fillMap(node *Node, code string, m map[byte]string) {
	if node.Left == node.Right {
		m[node.Data] = code
		return
	}

	arch.fillMap(node.Left, code+"1", m)
	arch.fillMap(node.Right, code+"0", m)
}

//////////////////////////////
func (arch *Archiver) buildTree(byteMeetCount [256]uint32) (*Node, error) {
	var nodePool []*Node
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

	if len(nodePool) == 0 {
		return nil, EmptyFileError
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
		nodePool = newNodePool
	}

	return nodePool[0], nil
}

func (arch *Archiver) buildMap(byteMeetCount [256]uint32) (map[byte]string, error) {

	root, err := arch.buildTree(byteMeetCount)
	if err != nil {
		return nil, err
	}

	m := make(map[byte]string)
	arch.fillMap(root, "", m)

	return m, nil
}

func (arhc *Archiver) writeEncoded(encoded *string, w io.Writer) error {
	var bytes []byte
	bytes, *encoded = bitString(*encoded).AsByteSlice()
	if len(bytes) == 0 {
		log.Print("+")
		return nil
	}
	log.Print("!@!", bytes)
	_, err := w.Write(bytes)
	return err
}

func (arch *Archiver) archiveFile(r io.Reader, byteMeetCount [256]uint32, w io.Writer) error {
	codeTable, err := arch.buildMap(byteMeetCount)
	if err != nil {
		return err
	}
	log.Print(codeTable)
	for _, count := range byteMeetCount {
		block := make([]byte, 4)
		binary.BigEndian.PutUint32(block[0:4], count)
		_, err = w.Write(block)
		if err != nil {
			return err
		}
	}

	var lastByteLen byte
	for byteVal, count := range byteMeetCount {
		add := byte(count) * byte(len(codeTable[byte(byteVal)]))
		lastByteLen = (lastByteLen + add) % 8
	}
	_, err = w.Write([]byte{lastByteLen})
	if err != nil {
		return err
	}

	var read int
	var encoded string
	readBuffer := make([]byte, BLOCK_SIZE)
	for err != io.EOF {
		read, err = r.Read(readBuffer)
		if err != nil && err != io.EOF {
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
		log.Print(encoded)
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

	byteMeetCount, err := arch.countBytes(file)
	if err != nil {
		return err
	}

	file.Seek(0, io.SeekStart)
	arch.archiveFile(file, byteMeetCount, output)

	return nil
}

func (arch *Archiver) readMeta(r io.Reader) ([256]uint32, byte, error) {
	buffer := make([]byte, 4)
	var byteMeetCount [256]uint32

	for i := 0; i < 256; i++ {
		_, err := r.Read(buffer)
		if err != nil {
			return [256]uint32{}, 0, err
		}

		byteMeetCount[i] = binary.BigEndian.Uint32(buffer)
	}

	buffer = make([]byte, 1)
	_, err := r.Read(buffer)
	if err != nil {
		return [256]uint32{}, 0, err
	}

	return byteMeetCount, buffer[0], nil
}

func nextNode(node *Node, code byte) (*Node, byte) {
	log.Print("@@", node.Weight)
	log.Print("@@@", string(code))
	if node == nil {
		return nil, 0
	}

	switch code {
	case '0':
		return node.Right, node.Right.Data
	case '1':
		return node.Left, node.Left.Data
	default:
		return nil, 0
	}
}

func (arhc *Archiver) decode(r io.Reader, root *Node, lastByteLength byte, w io.Writer) error {
	buffer := make([]byte, BLOCK_SIZE)
	writeBuffer := make([]byte, BLOCK_SIZE)
	writeIter := 0
	var bits string
	nodeIter := root

	var err error
	for err != io.EOF {
		var n int
		n, err = r.Read(buffer)
		if err != nil && err != io.EOF {
			return err
		}
		log.Print(buffer[:n])
		for i := 0; i < n; i++ {
			bits += toBitsAsString(buffer[i])
		}

		if err == io.EOF {
			var cutBits int
			cutBits = 8 - int(lastByteLength)
			if lastByteLength == 0 {
				cutBits = 0
			}
			bits = bits[:len(bits)-cutBits]
		}
		log.Print("!!!", bits)
		for len(bits) > 8 {
			var decoded byte
			nodeIter, decoded = nextNode(nodeIter, bits[0])
			if nodeIter.Left == nodeIter.Right {
				nodeIter = root

				writeBuffer[writeIter] = decoded
				writeIter++
				if writeIter == len(writeBuffer) {
					_, err = w.Write(writeBuffer)
					if err != nil {
						return err
					}
					writeIter = 0
				}
			}
			bits = bits[1:]
		}
	}
	for len(bits) > 0 {
		var decoded byte
		nodeIter, decoded = nextNode(nodeIter, bits[0])
		log.Print(string(decoded))
		if nodeIter.Left == nodeIter.Right {
			nodeIter = root

			writeBuffer[writeIter] = decoded
			writeIter++
			if writeIter == len(writeBuffer) {
				_, err = w.Write(writeBuffer)
				if err != nil {
					return err
				}
				writeIter = 0
			}
		}
		bits = bits[1:]
	}

	if writeIter != 0 {
		_, err = w.Write(writeBuffer[:writeIter])
		if err != nil {
			return err
		}
	}
	return nil
}

func (arch *Archiver) Decompress(r io.Reader, w io.Writer) error {
	byteMeetCount, lastByteLength, err := arch.readMeta(r)
	log.Print(byteMeetCount)
	log.Print(lastByteLength)

	root, err := arch.buildTree(byteMeetCount)
	if err != nil {
		return err
	}
	log.Print(root.Weight)

	arch.decode(r, root, lastByteLength, w)

	return nil
}

func NewArchiver() *Archiver {
	return &Archiver{}
}
