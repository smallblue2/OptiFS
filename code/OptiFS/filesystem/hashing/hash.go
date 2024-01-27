package hashing

import (
	"encoding/gob"
	"log"
	"os"
	"syscall"

	"lukechampine.com/blake3"
)

// hash in the release function
// take the data of the file
// pass it through the hashing function
// function to see if it is unique
// if hash's are the same, don't save the write, update reference count
// if they are different, proceed as normal

// key = inode number
// value = hash
var FileHashes = make(map[[64]byte]uint64)

func HashData(data []byte, flags uint32) [64]byte {

    // Check to see if we're appending
    if flags&syscall.O_APPEND != 0 {
        log.Println("APPENDING")
    }
    if flags&syscall.O_RDWR != 0 {
        log.Println("READWRITING")
    }
    if flags&syscall.O_WRONLY != 0 {
        log.Println("WRITINGONLY")
    }
    if flags&syscall.O_TRUNC != 0 {
        log.Println("TRUNCATING")
    }
    if flags&syscall.O_CREAT != 0 {
        log.Println("CREATING")
    }

	// get the hash
	hashResult := blake3.Sum512(data)

    log.Printf("HASHING: %s\nHASH: %x\n", string(data), hashResult)

	// finalHash := string(hashResult[:]) // needs to be stored as a string for the hashmap

	// log.Printf("Hash Converted to string: %v\n", finalHash) // issue!!!!

	return hashResult
}

// TODO: lookup func (isunique)
// TODO: hash evey file already in the system

// since a hashmap will be deleted when the system is restarted (stored in RAM)
// we encode the hashmap and store it in a file saved on disk to be loaded when OptiFS starts
func SaveMap(hashmap map[[64]byte]uint64) error {
	log.Println("SAVING HASHMAP")
	dest := "hashing/OptiFSHashSave.gob"

	// create the file if it doesn't exist, truncate it if it does
	// we assume nobody will be calling this file, as it is a very unique name
	file, err := os.Create(dest)

	if err != nil {
		log.Println("ERROR WITH FILE - HASHMAP")
		return err
	}

	defer file.Close() // don't let the file close

	encode := gob.NewEncoder(file) // set the file that we created to the encoder
	eErr := encode.Encode(hashmap) // encode the hashmap into binary, put it in the file

	if eErr != nil {
		log.Println("ERROR WITH ENCODER - HASHMAP")
		return eErr
	}

	return nil
}

// retrieve the encoded hashmap from the file when the system restarts
func RetrieveMap() error {
	log.Println("RETRIEVING HASHMAP")
	dest := "hashing/OptiFSHashSave.gob"

	file, err := os.Open(dest) // open where the hashmap was encoded

	if err != nil {
		return err
	}

	defer file.Close() // don't let the file close

	decode := gob.NewDecoder(file)     // set the file that we opened to the decoder
	dErr := decode.Decode(&FileHashes) // decode the file back into the hashmap

	if dErr != nil {
		log.Println("ERROR WITH DECODER - HASHMAP")
		return dErr
	}

	return nil
}

// printing hashmap for testing purposes
func PrintMap() {
	log.Println("PRINTING HASHMAP")
	for key, value := range FileHashes {
		log.Printf("Key: %v, Value: %x\n", key, value)
	}
}
