package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
)

func readRand(b []byte) (int, error) {
	return rand.Read(b)
}

func hmacSum(key, msg []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(msg)
	return h.Sum(nil)
}

func hmacSHA256(key, msg []byte) []byte {
	return hmacSum(key, msg)
}
