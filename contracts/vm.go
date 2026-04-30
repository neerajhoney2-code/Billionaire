package contracts

import (
	"encoding/binary"
	"fmt"
)

// Opcode is a single VM instruction byte.
type Opcode byte

const (
	OpPush   Opcode = 0x01 // PUSH <8-byte uint64 big-endian>
	OpPop    Opcode = 0x02 // POP  (discard top)
	OpAdd    Opcode = 0x03 // ADD  a + b
	OpSub    Opcode = 0x04 // SUB  a - b
	OpMul    Opcode = 0x05 // MUL  a * b
	OpDiv    Opcode = 0x06 // DIV  a / b
	OpStore  Opcode = 0x07 // STORE <key_len byte> <key bytes>
	OpLoad   Opcode = 0x08 // LOAD  <key_len byte> <key bytes> → pushes value (0 if missing)
	OpHalt   Opcode = 0xFF // HALT execution
)

// VM is the stack-based contract execution environment.
type VM struct {
	code    []byte
	pc      int
	stack   []uint64
	storage map[string]uint64 // contract storage
	gasLeft uint64
}

// NewVM creates a VM ready to execute bytecode with the given storage and gas.
func NewVM(code []byte, storage map[string]uint64, gas uint64) *VM {
	if storage == nil {
		storage = make(map[string]uint64)
	}
	return &VM{code: code, storage: storage, gasLeft: gas}
}

const gasCostOp uint64 = 1
const gasCostStorage uint64 = 10

// Execute runs the bytecode and returns the top-of-stack value (or 0).
func (vm *VM) Execute() (result uint64, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("vm panic: %v", r)
		}
	}()

	for vm.pc < len(vm.code) {
		op := Opcode(vm.code[vm.pc])
		vm.pc++

		if vm.gasLeft < gasCostOp {
			return 0, fmt.Errorf("out of gas")
		}
		vm.gasLeft -= gasCostOp

		switch op {
		case OpPush:
			if vm.pc+8 > len(vm.code) {
				return 0, fmt.Errorf("PUSH: not enough bytes")
			}
			val := binary.BigEndian.Uint64(vm.code[vm.pc : vm.pc+8])
			vm.pc += 8
			vm.push(val)

		case OpPop:
			vm.pop()

		case OpAdd:
			b, a := vm.pop(), vm.pop()
			vm.push(a + b)

		case OpSub:
			b, a := vm.pop(), vm.pop()
			if a < b {
				return 0, fmt.Errorf("SUB underflow")
			}
			vm.push(a - b)

		case OpMul:
			b, a := vm.pop(), vm.pop()
			vm.push(a * b)

		case OpDiv:
			b, a := vm.pop(), vm.pop()
			if b == 0 {
				return 0, fmt.Errorf("DIV by zero")
			}
			vm.push(a / b)

		case OpStore:
			key, err := vm.readKey()
			if err != nil {
				return 0, err
			}
			if vm.gasLeft < gasCostStorage {
				return 0, fmt.Errorf("out of gas for STORE")
			}
			vm.gasLeft -= gasCostStorage
			val := vm.pop()
			vm.storage[key] = val

		case OpLoad:
			key, err := vm.readKey()
			if err != nil {
				return 0, err
			}
			vm.push(vm.storage[key]) // 0 if missing

		case OpHalt:
			goto done

		default:
			return 0, fmt.Errorf("unknown opcode 0x%02X at pc=%d", op, vm.pc-1)
		}
	}

done:
	if len(vm.stack) > 0 {
		return vm.stack[len(vm.stack)-1], nil
	}
	return 0, nil
}

// Storage returns the mutated storage after execution.
func (vm *VM) Storage() map[string]uint64 {
	return vm.storage
}

func (vm *VM) push(v uint64) {
	vm.stack = append(vm.stack, v)
}

func (vm *VM) pop() uint64 {
	if len(vm.stack) == 0 {
		panic("stack underflow")
	}
	top := vm.stack[len(vm.stack)-1]
	vm.stack = vm.stack[:len(vm.stack)-1]
	return top
}

func (vm *VM) readKey() (string, error) {
	if vm.pc >= len(vm.code) {
		return "", fmt.Errorf("missing key length byte")
	}
	keyLen := int(vm.code[vm.pc])
	vm.pc++
	if vm.pc+keyLen > len(vm.code) {
		return "", fmt.Errorf("key bytes truncated")
	}
	key := string(vm.code[vm.pc : vm.pc+keyLen])
	vm.pc += keyLen
	return key, nil
}
