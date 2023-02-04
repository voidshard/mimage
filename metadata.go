package mimage

import (
	"encoding/json"
)

// metadata stores information on the massive image represented
// in it's array of smaller chunks so we can "load" an Mimage
// struct again
type metadata struct {
	BoundsMinX int
	BoundsMinY int
	BoundsMaxX int
	BoundsMaxY int
	ChunkSize  int
	Routines   int
}

// encodeJSON returns the JSON data representation of our metadata
func encodeJSON(m *metadata) ([]byte, error) {
	return json.Marshal(m)
}

// decodeJSON turns the JSON data representation into a metadata struct
func decodeJSON(data []byte) (*metadata, error) {
	m := &metadata{}
	return m, json.Unmarshal(data, m)
}
