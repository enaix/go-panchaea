package proto

import (
)

type Reply struct {
	Data string
	Error string
	ID int
	Bytecode []byte
}

type Receive struct {
	Data string
	Status string
	Error string
	ID int
	Thread int
	Bytecode []byte
}
