package main

import (
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"time"
)

type MapEntry struct {
    ReferenceCount uint32
    Nodes map[uint16]MapEntryNode
    UnderlyingInode uint32
}

type MapEntryNode struct {
    ReferenceNum uint32
    User uint16
    Metadata MapEntryNodeMetadata
    ExtendedData map[string]string
}

type MapEntryNodeMetadata struct {
    DeviceID uint16
    Mode uint8
    LinkCount uint16
    Size uint64
    ATime time.Time
    CTime time.Time
    MTime time.Time
    BlockSize uint16
    BlocksAllocated uint64
}

func encodeData() {
    tmpMeta1 := MapEntryNodeMetadata{DeviceID: 1, Mode: 243, LinkCount: 1, Size: 4096, ATime: time.Now(), CTime: time.Now(), MTime: time.Now(), BlockSize: 4096, BlocksAllocated: 1}
    tmpNode1 := MapEntryNode{ReferenceNum: 0, User: 100, Metadata: tmpMeta1, ExtendedData: make(map[string]string)}
    tmpEntry := MapEntry{ReferenceCount: 1, Nodes: make(map[uint16]MapEntryNode), UnderlyingInode: 25943}
    tmpEntry.Nodes[1] = tmpNode1

    log.Println("Gobbing:\n", tmpEntry)

    file, err := os.Create("myComplexData.gob")
    if err != nil {
        log.Fatal("Error creating the file: ", err)
    }
    defer file.Close()

    encoder := gob.NewEncoder(file)
    if err := encoder.Encode(tmpEntry); err != nil {
        log.Fatal("Error encoding the data: ", err)
    }

    log.Println("Gobbed data succesfully!")
}

func decodeData() {
    var myComplexStruct MapEntry
    file, err := os.Open("myComplexData.gob")
    if err != nil {
        log.Fatal("Error opening file 'myComplexData.gob': ", err)
    }
    defer file.Close()

    decoder := gob.NewDecoder(file)
    if err := decoder.Decode(&myComplexStruct); err != nil {
        log.Fatal("Error decoding file 'myComplexData.gob': ", err)
    }

    log.Println("Decoded data:\n", myComplexStruct)
}

func main() {
    var input string
    for input != "d" && input != "e" {
        fmt.Print("Encode or decode (e/d): ")
        fmt.Scanln(&input)
    }

    switch input {
    case "d":
        log.Println("Attempting to decode...")
        decodeData()
    case "e":
        log.Println("Attemping to encode...")
        encodeData()
    }
}
