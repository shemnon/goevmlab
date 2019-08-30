package evms

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/core/vm"
	"io"
	"os/exec"
	"sync"
)

// GethEVM is s Evm-interface wrapper around the `evm` binary, based on go-ethereum.
type GethEVM struct {
	path string
	wg   sync.WaitGroup
}

func NewGethEVM(path string) *GethEVM {
	return &GethEVM{
		path: path,
	}
}

// StartStateTest implements the Evm interface
func (evm *GethEVM) StartStateTest(path string) (chan *vm.StructLog, error) {
	var (
		stderr io.ReadCloser
		err    error
	)
	cmd := exec.Command(evm.path, "--json", "--nomemory", "statetest", path)
	if stderr, err = cmd.StderrPipe(); err != nil {
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	ch := make(chan *vm.StructLog)
	evm.wg.Add(1)
	go evm.feed(stderr, ch)
	return ch, nil

}

func (vm *GethEVM) Close() {
	vm.wg.Wait()
}

// feed reads from the reader, does some geth-specific filtering and
// outputs items onto the channel
func (evm *GethEVM) feed(input io.Reader, opsCh chan (*vm.StructLog)) {
	defer close(opsCh)
	defer evm.wg.Done()
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		// Calling bytes means that bytes in 'l' will be overwritten
		// in the next loop. Fine for now though, we immediately marshal it
		data := scanner.Bytes()
		var elem vm.StructLog
		err := json.Unmarshal(data, &elem)
		if err != nil {
			fmt.Printf("geth err: %v, line\n\t%v\n", err, string(data))
			continue
		}
		// If the output cannot be marshalled, all fields will be blanks.
		// We can detect that through 'depth', which should never be less than 1
		// for any actual opcode
		if elem.Depth == 0 {
			/*  Most likely one of these:
			{"output":"","gasUsed":"0x2d1cc4","time":233624,"error":"gas uint64 overflow"}

			{"stateRoot": "a2b3391f7a85bf1ad08dc541a1b99da3c591c156351391f26ec88c557ff12134"}

			*/
			fmt.Printf("geth non-op, line s:\n\t%v\n", string(data))
			// For now, just ignore these
			continue
		}
		opsCh <- &elem
	}
}

func has0xPrefix(input string) bool {
	return len(input) >= 2 && input[0] == '0' && (input[1] == 'x' || input[1] == 'X')
}

// addPrefix prefixes hexStr with 0x, if needed
func addPrefix(hexStr string) string {
	if len(hexStr) >= 2 && hexStr[1] != 'x' {
		enc := make([]byte, len(hexStr)+2)
		copy(enc, "0x")
		copy(enc[2:], hexStr)
		return string(enc)
	}
	return hexStr
}