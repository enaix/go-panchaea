package main

import (
	"encoding/json"
	"errors"
	"fmt"
)

type Result struct {
	Sum float64
}

type WorkUnit struct {
	Start int
	End   int
	Sign  int
}

type Server struct {
	Current       int // Current client
	PrepareAmount int // Amount of WUs to be generated
	//	WorkUnits []*WorkUnit // List of prepared WUs
	Custom []byte // Custom server data, stored in JSON
}

func (s *Server) Init() {
	s.Current = 0
	s.PrepareAmount = 10
}

func (s *Server) Run(id int) ([]byte, error) {
	if len(s.WorkUnits) == 0 {
		err := s.Prepare(s.PrepareAmount)
		if err != nil {
			return nil, err
		}
	}
	res := s.WorkUnits[0]
	wu, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}
	s.WorkUnits = s.WorkUnits[1:]
	s.Current++
	return wu, nil
}

func (s *Server) Prepare(amount int) error {
	if s.Current >= 1000000 {
		return errors.New("No units to generate")
	}
	for i := 0; i < amount; i++ {
		var wu = WorkUnit{Start: 100000 * (s.Current + i), End: 100000 + 100000*(s.Current+i)}
		s.WorkUnits = append(s.WorkUnits, &wu)
	}
	return nil
}

func (s *Server) Process(res [][]byte) error {
	comp := make([]Result, 0)
	for _, v := range res {
		var r Result
		err := json.Unmarshal(v, &r)
		if err != nil {
			return err
		}
		comp = append(comp, r)
	}
	var sum float64
	for _, v := range comp {
		sum += v.Sum
	}
	fmt.Println(sum)
	// TODO add output to file
	return nil
}
