package hashing

import (
	"fmt"

	"lukechampine.com/blake3"
)

// hash in the release function
// take the data of the file
// pass it through the hashing function
// function to see if it is unique
// if hash's are the same, don't save the write, update reference count
// if they are different, proceed as normal

func HashData(data []byte) []byte {

	// get the hash
	hashResult := blake3.Sum512(data)

	// need to use %x to format each byte as a hex string
	fmt.Printf("Hash of that string: %x\n", hashResult)

	return hashResult[:]
}
