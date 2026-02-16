// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package interfaces

// MapType represents different eBPF map types
type MapType int

const (
	MapTypeUnspec MapType = iota
	MapTypeHash
	MapTypeArray
	MapTypeProgArray
	MapTypePerfEventArray
	MapTypePerCPUHash
	MapTypePerCPUArray
	MapTypeStackTrace
	MapTypeCgroupArray
	MapTypeLRUHash
	MapTypeLRUPerCPUHash
	MapTypeLPMTrie
	MapTypeArrayOfMaps
	MapTypeHashOfMaps
	MapTypeDevmap
	MapTypeSockmap
	MapTypeCPUMap
	MapTypeXSKMap
	MapTypeSockHash
	MapTypeCgroupStorage
	MapTypeReusePortSockArray
	MapTypePerCPUCgroupStorage
	MapTypeQueue
	MapTypeStack
	MapTypeSkStorage
	MapTypeDevmapHash
	MapTypeStructOps
	MapTypeRingbuf
	MapTypeInodeStorage
	MapTypeTaskStorage
)

// MapIterator interface for iterating over map entries
type MapIterator interface {
	Next() (bool, error)
	NextKey() (interface{}, error)
	NextValue() (interface{}, error)
	Close() error
}
