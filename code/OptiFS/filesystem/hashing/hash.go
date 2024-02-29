package hashing

import (
	"log"

	"lukechampine.com/blake3"
)

// Simply hashes the data provided using BLAKE3 and returns a 64-byte hash.
func HashContents(data []byte, flags uint32) [64]byte {

    // TODO: Behave differently depending on open flag, specifically appending Or maybe not, seems to work perfectly????

	//if flags&syscall.O_APPEND != 0 {
	//	log.Println("APPENDING")
	//}
	//if flags&syscall.O_RDWR != 0 {
	//	log.Println("READWRITING")
	//}
	//if flags&syscall.O_WRONLY != 0 {
	//	log.Println("WRITINGONLY")
	//}
	//if flags&syscall.O_TRUNC != 0 {
	//	log.Println("TRUNCATING")
	//}
	//if flags&syscall.O_CREAT != 0 {
	//	log.Println("CREATING")
	//}

    log.Println("Hashing data...")

	return blake3.Sum512(data)
}
