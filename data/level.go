package data

var Level = newLevelRegistry()

type levelRegistry struct {
	/* House of Representatives Member*/
	FedRep string
	/* Federal FedSenator */
	FedSenator   string
	StateRep     string
	StateSenator string
}

func newLevelRegistry() *levelRegistry {
	return &levelRegistry{
		FedRep:       "Federal House of Representatives Member",
		FedSenator:   "Federal Senator",
		StateRep:     "State Mp",
		StateSenator: "State Senator",
	}
}
