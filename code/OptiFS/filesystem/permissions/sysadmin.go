package permissions

import (
	"encoding/gob"
	"log"
	"os"
)

type Sysadmin struct {
	UID uint32
	GID uint32
	Set bool // flag to see if we have set the sysadmin already
}

var SysAdmin Sysadmin

// save the sysadmin details when the system shuts down
func SaveSysadmin() error {
	dest := "permissions/OptiFSSysadminSave.gob"

	// create the file if it doesn't exist, truncate it if it does
	// we assume nobody will be calling this file, as it is a very unique name
	file, err := os.Create(dest)

	if err != nil {
		log.Println("ERROR WITH FILE - SYSADMIN")
		return err
	}

	defer file.Close() // don't let the file close

	encode := gob.NewEncoder(file)  // set the file that we created to the encoder
	eErr := encode.Encode(SysAdmin) // encode the hashmap into binary, put it in the file

	if eErr != nil {
		log.Println("ERROR WITH ENCODER - SYSADMIN")
		return eErr
	}

	return nil

}

// retrieve the sysadmin details when the system boots up
func RetrieveSysadmin() error {
	dest := "permissions/OptiFSSysadminSave.gob"

	file, err := os.Open(dest) // open where the info was encoded

	if err != nil {
		return err
	}

	defer file.Close() // don't let file close

	decode := gob.NewDecoder(file)   // set the file that we opened to the decoder
	dErr := decode.Decode(&SysAdmin) // decode the file back into the struct

	if dErr != nil {
		log.Println("ERROR WITH DECODER - SYSADMIN")
		return dErr
	}

	return nil

}

// print the current sysadmin
func PrintSysadminInfo() {
	log.Printf("Current SysAdmin: %+v\n", SysAdmin)
}
