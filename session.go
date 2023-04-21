package main

type session interface {
	close() error
	write(mainType uint16, subType uint16, packet interface{}) error
}
