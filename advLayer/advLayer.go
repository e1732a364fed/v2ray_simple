//Package advLayer contains subpackages for Advanced Layer in VSI model.
package advLayer

import (
	"fmt"

	"github.com/hahahrfool/v2ray_simple/utils"
)

var ProtocolsMap = make(map[string]bool)

func PrintAllProtocolNames() {
	fmt.Printf("===============================\nSupported Advanced Layer protocols:\n")
	for _, v := range utils.GetMapSortedKeySlice(ProtocolsMap) {
		fmt.Print(v)
		fmt.Print("\n")
	}
}
