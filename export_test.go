package gollem

var NewDefaultFacilitator = newDefaultFacilitator

func (x *Agent) Facilitator() Facilitator {
	return x.gollemConfig.facilitator
}
