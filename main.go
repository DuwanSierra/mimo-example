package main

import (
	"fmt"
	"io"
	"math/big"
	"os"
	"sync"
	"time"
)

type Pdu struct {
	point Point
	id    int
}

type Signal struct {
	// Define properties of a Signal here
	data Pdu
}

func transmitter(tx []chan Signal, wg *sync.WaitGroup, pdu Pdu) {
	defer wg.Done()
	// Simulate transmitting a signal
	for _, ch := range tx {
		//fmt.Println("Transmitting signal: ", pdu.id)
		ch <- Signal{
			data: pdu,
		}
	}
}

func receiver(rx []chan Signal, wg *sync.WaitGroup, pduDictionary *[]Pdu) {
	defer wg.Done()
	// Simulate receiving a signal
	for _, ch := range rx {
		data := <-ch
		//fmt.Println("Data received", data.data.id)
		*pduDictionary = append(*pduDictionary, data.data)
	}
}

func main() {
	defer timeTrack(time.Now(), "Modulation and Demodulation")
	pathFile := "input_video.mp4"
	level := 64
	noise := 0.20
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
		pduDictionary := make([]Pdu, 0)
		//Iterate all points and send them to the antennas
		for count, point := range points {
			pdu := Pdu{point, count}
			for i := 0; i < txAntenna; i++ {
				wg.Add(1)
				go transmitter(matrix[i], &wg, pdu)
			}

			for i := 0; i < rxAntenna; i++ {
				rx := make([]chan Signal, txAntenna)
				for j := 0; j < txAntenna; j++ {
					rx[j] = matrix[j][i]
				}
				wg.Add(1)
				go receiver(rx, &wg, &pduDictionary)
			}
		}

		wg.Wait()
		pduMap := orderPdu(pduDictionary)
		restorePoints := createPduFromAverageOfPdu(pduMap)
		bitsRestore := demodulate(restorePoints, M, noise)
		originalBytes := bitsToBytes(bitsRestore)
		writeRestoreFile("restore_"+pathFile, originalBytes)
	}
	fmt.Println("Successfully demodulated the data!")
}

func orderPdu(pduDictionary []Pdu) map[int][]Pdu {
	//Order the pduDictionary
	pduMap := make(map[int][]Pdu)
	for _, pdu := range pduDictionary {
		pduMap[pdu.id] = append(pduMap[pdu.id], pdu)
	}
	return pduMap
}

func createPduFromAverageOfPdu(pdus map[int][]Pdu) []Point {
	points := make([]Point, 0, len(pdus))
	for _, pduList := range pdus {
		var x int64 = 0
		var y int64 = 0
		for _, pdu := range pduList {
			x += pdu.point.x
			y += pdu.point.y
		}
		xAverage := x / int64(len(pduList))
		yAverage := y / int64(len(pduList))
		points = append(points, Point{xAverage, yAverage})
	}
	return points

}
