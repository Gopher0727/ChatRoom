package proto

import (
	"fmt"
	"log"
	"os"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const fname = "addressbook.bin"

func TestProto(t *testing.T) {
	// 构造 AddressBook 并写入文件
	book := &AddressBook{}

	p := &Person{
		Id:    1234,
		Name:  "John Doe",
		Email: "jdoe@example.com",
		Phones: []*Person_PhoneNumber{
			{Number: "555-4321", Type: PhoneType_PHONE_TYPE_HOME},
		},
	}
	book.People = append(book.People, p)

	Write(fname, book)

	b := Read(fname)

	mode := protojson.MarshalOptions{
		Multiline: true,
		Indent:    "    ",
	}
	js, err := mode.Marshal(b)
	if err != nil {
		t.Fatalf("JSON Marshal failed: %v", err)
	}
	fmt.Println("Read address book (JSON):")
	fmt.Println(string(js))
}

// Write 将 AddressBook 二进制写入 fname
func Write(fname string, book *AddressBook) {
	out, err := proto.Marshal(book)
	if err != nil {
		log.Fatalln("Failed to encode address book:", err)
	}
	if err := os.WriteFile(fname, out, 0o644); err != nil {
		log.Fatalln("Failed to write address book:", err)
	}
}

// Read 从 fname 读取二进制并反序列化为 AddressBook
func Read(fname string) *AddressBook {
	in, err := os.ReadFile(fname)
	if err != nil {
		log.Fatalln("Error reading file:", err)
	}
	book := &AddressBook{}
	if err := proto.Unmarshal(in, book); err != nil {
		log.Fatalln("Failed to parse address book:", err)
	}
	return book
}
