package main

import (
	"fmt"
	"io"
	"math/big"
	"os"
	"sync"
	"time"
)

type Signal struct {
	// Define properties of a Signal here
}

type Pdu struct {
	point Point
	id    int
}

func transmitter(tx []chan Signal, wg *sync.WaitGroup, id int, pdu Pdu) {
	defer wg.Done()
	// Simulate transmitting a signal
	for i, ch := range tx {
		ch <- Signal{}
		fmt.Printf("Transmitter %d sent a signal to Receiver %d with pdu id:%d \n", id, i, pdu.id)
	}
}

func receiver(rx []chan Signal, wg *sync.WaitGroup, id int, pdu Pdu, pduDictionary *map[int][]Point) {
	defer wg.Done()
	// Simulate receiving a signal
	for i, ch := range rx {
		<-ch
		fmt.Printf("Receiver %d received a signal from Transmitter %d with pdu id:%d \n", id, i, pdu.id)
		(*pduDictionary)[pdu.id] = append((*pduDictionary)[pdu.id], pdu.point)
	}
}

func main() {
	defer timeTrack(time.Now(), "Modulation and Demodulation")
	pathFile := "input_video.mp4"
	level := 64
	//noise := 0.20
	chunkSize := 8192 // size of each chunk in bytes

	M := new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(level)), nil)
	fmt.Println("Modulation level M: ", M)

	file, err := os.Open(pathFile)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	data := make([]byte, chunkSize)

	//Antennas
	var wg sync.WaitGroup
	rxAntenna := 2
	txAntenna := 2

	// Create a matrix of Tx to Rx connections
	matrix := make([][]chan Signal, txAntenna)
	for i := range matrix {
		matrix[i] = make([]chan Signal, rxAntenna)
		for j := range matrix[i] {
			matrix[i][j] = make(chan Signal)
		}
	}

	for {
		_, err := io.ReadFull(file, data)
		if err == io.EOF {
			break
		} else if err != nil && err != io.ErrUnexpectedEOF {
			fmt.Println("Error reading file:", err)
			return
		}

		bits := bytesToBits(data)
		points := modulate(bits, M)
		pduDictionary := make(map[int][]Point)
		//Iterate all points and send them to the antennas
		for count, point := range points {
			pdu := Pdu{point, count}
			for i := 0; i < txAntenna; i++ {
				wg.Add(1)
				go transmitter(matrix[i], &wg, i, pdu)
			}

			for i := 0; i < rxAntenna; i++ {
				rx := make([]chan Signal, txAntenna)
				for j := 0; j < txAntenna; j++ {
					rx[j] = matrix[j][i]
				}
				wg.Add(1)
				go receiver(rx, &wg, i, pdu, &pduDictionary)
			}
		}

		wg.Wait()

		//Print the pduDictionary
		fmt.Println(pduDictionary)
		break
		/*bitsRestore := demodulate(pathModulatePoints, M, noise)
		originalBytes := bitsToBytes(bitsRestore)
		writeRestoreFile("restore_"+pathFile, originalBytes)*/
	}
	fmt.Println("Successfully demodulated the data!")
}
