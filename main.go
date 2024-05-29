package main

import (
	"fmt"
	"io"
	"math/big"
	"os"
	"sort"
	"sync"
	"time"
)

// Job represents a task to be done
type Job struct {
	pdu      Pdu
	channels []chan Signal
}

// Worker represents a transmitter or receiver
type Worker struct {
	wg                *sync.WaitGroup
	jobs              <-chan Job
	safePduDictionary *SafePduDictionary
	isReceiver        bool
}

// NewWorker creates a new Worker
func NewWorker(wg *sync.WaitGroup, jobs <-chan Job, safePduDictionary *SafePduDictionary, isReceiver bool) *Worker {
	return &Worker{wg, jobs, safePduDictionary, isReceiver}
}

// Start starts the Worker
func (w *Worker) Start() {
	go func() {
		for job := range w.jobs {
			if w.isReceiver {
				w.receiver(job)
			} else {
				w.transmitter(job)
			}
			w.wg.Done()
		}
	}()
}

// transmitter simulates transmitting a signal
func (w *Worker) transmitter(job Job) {
	for _, ch := range job.channels {
		ch <- Signal{
			data: job.pdu,
		}
	}
}

// receiver simulates receiving a signal
func (w *Worker) receiver(job Job) {
	for _, ch := range job.channels {
		data := <-ch
		w.safePduDictionary.Append(data.data)
	}
}

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

// Reset resets the SafePduDictionary to its initial state
func (spd *SafePduDictionary) Reset() {
	spd.mu.Lock()
	defer spd.mu.Unlock()

	// Reset the internal state of the SafePduDictionary
	// This depends on how your SafePduDictionary is implemented
	// For example, if it contains a map, you can do:
	spd.pdus = []Pdu(nil)
}

func main() {
	defer timeTrack(time.Now(), "Modulation and Demodulation")
	pathFile := "input_video.mp4"
	level := 64
	noise := 0.65
	chunkSize := 100000 // size of each chunk in bytes in this case are 1 Mb
	restoreFileName := "restore_" + pathFile

	removeFileIfExists(restoreFileName)
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
	rxAntenna := 16
	txAntenna := 16
	// Create job channels for transmitters and receivers
	txJobs := make(chan Job, txAntenna)
	rxJobs := make(chan Job, rxAntenna)
	safePduDictionary := &SafePduDictionary{}

	// Create workers
	for i := 0; i < txAntenna; i++ {
		worker := NewWorker(&wg, txJobs, nil, false)
		worker.Start()
	}
	for i := 0; i < rxAntenna; i++ {
		worker := NewWorker(&wg, rxJobs, safePduDictionary, true)
		worker.Start()
	}

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
			fmt.Println("End of file")
			break
		} else if err != nil && err != io.ErrUnexpectedEOF {
			fmt.Println("Error reading file:", err)
			return
		}

		bits := bytesToBits(data)
		//Hamming
		hammingBits := make([]int64, 0, len(bits)/4*7)
		for i := 0; i < len(bits); i += 4 {
			end := i + 4
			if end > len(bits) {
				end = len(bits)
			}
			hammingBits = append(hammingBits, addHammingParityBits(bits[i:end])...)
		}
		points := modulate(hammingBits, M)
		//Iterate all points and send them to the antennas
		for count, point := range points {
			pdu := Pdu{point, count}
			for i := 0; i < txAntenna; i++ {
				wg.Add(1)
				pduWithNoise := addNoiseToPdu(pdu, noise)
				txJobs <- Job{pduWithNoise, matrix[i]}
			}

			for i := 0; i < rxAntenna; i++ {
				rx := make([]chan Signal, txAntenna)
				for j := 0; j < txAntenna; j++ {
					rx[j] = matrix[j][i]
				}
				wg.Add(1)
				rxJobs <- Job{pdu, rx}
			}
		}
		wg.Wait()
		sort.Slice(safePduDictionary.pdus, func(i, j int) bool {
			return safePduDictionary.pdus[i].Index < safePduDictionary.pdus[j].Index
		})
		pdusRestore := createPduFromMostRepeatedPdu(safePduDictionary.pdus)

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
		correctedBits := make([]int64, 0, len(bitsRestore)/7*4)
		for i := 0; i < len(bitsRestore); i += 7 {
			end := i + 7
			if end > len(bitsRestore) {
				end = len(bitsRestore)
			}
			correctedBits = append(correctedBits, checkAndCorrectHammingCode(bitsRestore[i:end])...)
		}
		originalBytes := bitsToBytes(correctedBits)
		writeRestoreFile(restoreFileName, originalBytes)
		safePduDictionary.Reset()
	}

	close(txJobs)
	close(rxJobs)
	fmt.Println("Successfully demodulated the data!")
}

func createPduFromMostRepeatedPdu(pdus []Pdu) []Pdu {
	results := make(chan Pdu, len(pdus))
	points := make([]Pdu, 0, len(pdus))
	var wg sync.WaitGroup

	//group by id
	groups := make(map[int][]Pdu)
	for _, pdu := range pdus {
		groups[pdu.Index] = append(groups[pdu.Index], pdu)
	}

	//find the most repeated pdu
	for _, group := range groups {
		wg.Add(1)
		go func(group []Pdu) {
			defer wg.Done()

			pduCounts := make(map[Pdu]int)
			maxCount := 0

			for _, pdu := range group {
				pduCounts[pdu]++
				if pduCounts[pdu] > maxCount {
					maxCount = pduCounts[pdu]
				}
			}

			var mostRepeatedPdus []Pdu
			for pdu, count := range pduCounts {
				if count == maxCount {
					mostRepeatedPdus = append(mostRepeatedPdus, pdu)
				}
			}

			var result Pdu
			if len(mostRepeatedPdus) == 1 {
				result = mostRepeatedPdus[0]
			} else {
				fmt.Println("Multiple most repeated pdus found, averaging them")
				var x, y int64
				for _, pdu := range mostRepeatedPdus {
					x += pdu.point.x
					y += pdu.point.y
				}
				x /= int64(len(mostRepeatedPdus))
				y /= int64(len(mostRepeatedPdus))
				result = Pdu{Point{x, y}, mostRepeatedPdus[0].Index}
			}

			results <- result
		}(group)
	}

	wg.Wait()
	close(results)

	for result := range results {
		points = append(points, result)
	}

	return points
}
