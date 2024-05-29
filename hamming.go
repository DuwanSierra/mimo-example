package main

import (
	"fmt"
	"math/big"
	"math/rand"
	"time"
)

// Función para agregar bits de paridad de Hamming
func addHammingParityBits(data []int64) []int64 {
	n := len(data)
	m := 0
	for (1 << m) < n+m+1 {
		m++
	}
	hammingCode := make([]int64, n+m)

	// Inicializar los bits de datos y paridad
	j, _ := 0, 0
	for i := 1; i <= len(hammingCode); i++ {
		if (i & (i - 1)) == 0 {
			hammingCode[i-1] = 0
		} else {
			hammingCode[i-1] = data[j]
			j++
		}
	}

	// Calcular los bits de paridad
	for i := 0; i < m; i++ {
		pos := 1 << i
		parity := int64(0)
		for j := 1; j <= len(hammingCode); j++ {
			if j&pos != 0 {
				parity ^= hammingCode[j-1]
			}
		}
		hammingCode[pos-1] = parity
	}

	return hammingCode
}

// Función para verificar y corregir el código Hamming
func checkAndCorrectHammingCode(data []int64) []int64 {
	n := len(data)
	m := 0
	for (1 << m) < n {
		m++
	}

	errorPos := 0
	for i := 0; i < m; i++ {
		pos := 1 << i
		parity := int64(0)
		for j := 1; j <= n; j++ {
			if j&pos != 0 {
				parity ^= data[j-1]
			}
		}
		if parity != 0 {
			errorPos |= pos
		}
	}

	if errorPos != 0 {
		data[errorPos-1] ^= 1
	}

	// Eliminar los bits de paridad
	correctedData := make([]int64, n-m)
	j := 0
	for i := 1; i <= n; i++ {
		if (i & (i - 1)) != 0 {
			correctedData[j] = data[i-1]
			j++
		}
	}

	return correctedData
}

func ola() {
	rand.Seed(time.Now().UnixNano())

	data := []byte("Hola, mundo!")
	bits := bytesToBits(data)

	hammingBits := make([]int64, 0, len(bits)/4*7)
	for i := 0; i < len(bits); i += 4 {
		end := i + 4
		if end > len(bits) {
			end = len(bits)
		}
		hammingBits = append(hammingBits, addHammingParityBits(bits[i:end])...)
	}

	M := big.NewInt(256)
	points := modulate(hammingBits, M)

	pdus := make([]Pdu, len(points))
	for i, point := range points {
		pdus[i] = addNoiseToPdu(Pdu{point: point}, 0.1)
	}

	var receivedPoints []Point
	for _, pdu := range pdus {
		receivedPoints = append(receivedPoints, pdu.point)
	}
	receivedBits := demodulate(receivedPoints, M)

	correctedBits := make([]int64, 0, len(receivedBits)/7*4)
	for i := 0; i < len(receivedBits); i += 7 {
		end := i + 7
		if end > len(receivedBits) {
			end = len(receivedBits)
		}
		correctedBits = append(correctedBits, checkAndCorrectHammingCode(receivedBits[i:end])...)
	}

	receivedData := bitsToBytes(correctedBits)

	fmt.Println("Data recibida:", string(receivedData))
}
