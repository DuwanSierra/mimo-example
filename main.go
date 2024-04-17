package main

import (
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"sort"
	"sync"
	"time"
)

type Pdu struct {
	point Point
	Index int
}

type Signal struct {
	// Define properties of a Signal here
	data Pdu
}

type SafePduDictionary struct {
	mu   sync.Mutex
	pdus []Pdu
}

func (s *SafePduDictionary) Append(pdu Pdu) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pdus = append(s.pdus, pdu)
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

func receiver(rx []chan Signal, wg *sync.WaitGroup, safePduDictionary *SafePduDictionary) {
	defer wg.Done()
	// Simulate receiving a signal
	for _, ch := range rx {
		data := <-ch
		safePduDictionary.Append(data.data)
	}
}

func main() {
	defer timeTrack(time.Now(), "Modulation and Demodulation")
	pathFile := "input_video.mp4"
	level := 64
	noise := 0.2
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
	rxAntenna := 10
	txAntenna := 10

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
		safePduDictionary := &SafePduDictionary{}
		//Iterate all points and send them to the antennas
		for count, point := range points {
			pdu := Pdu{point, count}
			for i := 0; i < txAntenna; i++ {
				wg.Add(1)
				go transmitter(matrix[i], &wg, addNoiseToPdu(pdu, noise))
			}

			for i := 0; i < rxAntenna; i++ {
				rx := make([]chan Signal, txAntenna)
				for j := 0; j < txAntenna; j++ {
					rx[j] = matrix[j][i]
				}
				wg.Add(1)
				go receiver(rx, &wg, safePduDictionary)
			}
		}

		wg.Wait()
		sort.Slice(safePduDictionary.pdus, func(i, j int) bool {
			return safePduDictionary.pdus[i].Index < safePduDictionary.pdus[j].Index
		})
		pdusRestore := createPduFromAverageOfPdu(safePduDictionary.pdus)

		//order restore points by index
		sort.Slice(pdusRestore, func(i, j int) bool {
			return pdusRestore[i].Index < pdusRestore[j].Index
		})

		//Restore points are the pdus restored in point
		restorePoints := make([]Point, 0, len(pdusRestore)*2)
		for _, pdu := range pdusRestore {
			restorePoints = append(restorePoints, pdu.point)
		}
		bitsRestore := demodulate(restorePoints, M)
		originalBytes := bitsToBytes(bitsRestore)
		writeRestoreFile("restore_"+pathFile, originalBytes)
	}
	fmt.Println("Successfully demodulated the data!")
}

func createPduFromAverageOfPdu(pdus []Pdu) []Pdu {
	const threshold = 1.0
	points := make([]Pdu, 0, len(pdus))
	//group by id
	groups := make(map[int][]Pdu)
	for _, pdu := range pdus {
		groups[pdu.Index] = append(groups[pdu.Index], pdu)
	}
	//average of points
	for _, group := range groups {
		var x int64 = 0
		var y int64 = 0
		for _, pdu := range group {
			x += pdu.point.x
			y += pdu.point.y
		}

		xMean := x / int64(len(group))
		yMean := y / int64(len(group))

		var xSumSquares, ySumSquares float64
		for _, pdu := range group {
			xDiff := float64(pdu.point.x) - float64(xMean)
			yDiff := float64(pdu.point.y) - float64(yMean)
			xSumSquares += xDiff * xDiff
			ySumSquares += yDiff * yDiff
		}

		xStdDev := math.Sqrt(xSumSquares / float64(len(group)))
		yStdDev := math.Sqrt(ySumSquares / float64(len(group)))

		if xStdDev > threshold || yStdDev > threshold {
			fmt.Printf("High standard deviation for group %d: xStdDev = %f, yStdDev = %f\n", group[0].Index, xStdDev, yStdDev)
			fmt.Println(group)
		}

		points = append(points, Pdu{point: Point{xMean, yMean}, Index: group[0].Index})
	}
	return points
}
