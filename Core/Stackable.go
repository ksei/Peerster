package core

type Stackable interface {
	GetOrigin() string
	GetID() uint32
	GetValue() interface{}
}

func (rumor *RumourMessage) GetOrigin() string {
	return rumor.Origin
}

func (rumor *RumourMessage) GetID() uint32 {
	return rumor.ID
}

func (rumor *RumourMessage) GetValue() interface{} {
	return rumor.Text
}

func (tlc *TLCMessage) GetOrigin() string {
	return tlc.Origin
}

func (tlc *TLCMessage) GetID() uint32 {
	return tlc.ID
}

func (tlc *TLCMessage) GetValue() interface{} {
	return tlc
}
