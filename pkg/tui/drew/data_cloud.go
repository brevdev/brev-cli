package drew

type Cloud int

const (
	CloudCrusoe Cloud = iota
	CloudLambda
)

var cloudName = map[Cloud]string{
	CloudCrusoe: "Crusoe",
	CloudLambda: "Lambda",
}

func (c Cloud) Name() string {
	return cloudName[c]
}
