package hashing

import (
	"encoding/gob"
	"fmt"
	"log"
	"os"

	"lukechampine.com/blake3"
)

// hash in the release function
// take the data of the file
// pass it through the hashing function
// function to see if it is unique
// if hash's are the same, don't save the write, update reference count
// if they are different, proceed as normal

var FileHashes = make(map[string][]byte)

func HashData(data []byte) []byte {

	// get the hash
	hashResult := blake3.Sum512(data)

	// need to use %x to format each byte as a hex string
	fmt.Printf("Hash of that string: %x\n", hashResult)

	return hashResult[:]
}

// lookup func (isunique)

// since a hashmap will be deleted when the system is restarted (stored in RAM)
// we encode the hashmap and store it in a file saved on disk to be loaded when OptiFS starts
func SaveMap(hashmap map[string][]byte) error {
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
