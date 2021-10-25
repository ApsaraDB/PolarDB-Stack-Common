/*
*Copyright (c) 2019-2021, Alibaba Group Holding Limited;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
*You may obtain a copy of the License at

*   http://www.apache.org/licenses/LICENSE-2.0

*Unless required by applicable law or agreed to in writing, software
*distributed under the License is distributed on an "AS IS" BASIS,
*WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*See the License for the specific language governing permissions and
*limitations under the License.
 */

package adapter

import (
	"fmt"
	"strconv"
	"strings"
)

type PortRange struct {
	Base int
	Size int
}

func (pr *PortRange) Contains(p int) bool {
	return (p >= pr.Base) && ((p - pr.Base) < pr.Size)
}

func (pr PortRange) String() string {
	if pr.Size == 0 {
		return ""
	}
	return fmt.Sprintf("%d-%d", pr.Base, pr.Base+pr.Size-1)
}

func (pr *PortRange) Set(value string) error {
	const (
		SinglePortNotation = 1 << iota
		HyphenNotation
		PlusNotation
	)

	value = strings.TrimSpace(value)
	hyphenIndex := strings.Index(value, "-")
	plusIndex := strings.Index(value, "+")

	if value == "" {
		pr.Base = 0
		pr.Size = 0
		return nil
	}

	var err error
	var low, high int
	var notation int

	if plusIndex == -1 && hyphenIndex == -1 {
		notation |= SinglePortNotation
	}
	if hyphenIndex != -1 {
		notation |= HyphenNotation
	}
	if plusIndex != -1 {
		notation |= PlusNotation
	}

	switch notation {
	case SinglePortNotation:
		var port int
		port, err = strconv.Atoi(value)
		if err != nil {
			return err
		}
		low = port
		high = port
	case HyphenNotation:
		low, err = strconv.Atoi(value[:hyphenIndex])
		if err != nil {
			return err
		}
		high, err = strconv.Atoi(value[hyphenIndex+1:])
		if err != nil {
			return err
		}
	case PlusNotation:
		var offset int
		low, err = strconv.Atoi(value[:plusIndex])
		if err != nil {
			return err
		}
		offset, err = strconv.Atoi(value[plusIndex+1:])
		if err != nil {
			return err
		}
		high = low + offset
	default:
		return fmt.Errorf("unable to parse port range: %s", value)
	}

	//if low > 65535 || high > 65535 {
	//	return fmt.Errorf("the port range cannot be greater than 65535: %s", value)
	//}

	if high < low {
		return fmt.Errorf("end port cannot be less than start port: %s", value)
	}

	pr.Base = low
	pr.Size = 1 + high - low
	return nil
}

func (*PortRange) Type() string {
	return "portRange"
}

func ParsePortRange(value string) (*PortRange, error) {
	pr := &PortRange{}
	err := pr.Set(value)
	if err != nil {
		return nil, err
	}
	return pr, nil
}

type PortRanges struct {
	PortRange          *PortRange
	ReservedPortRanges []*PortRange
}

func (prs *PortRanges) Set(value string) error {

	pr, err := ParsePortRange(value)
	if err != nil {
		return err
	}
	prs.PortRange = pr
	return nil
}
func (prs *PortRanges) SetReservedPortRanges(value string) error {
	reservedPortRanges := strings.Split(value, ",")
	for _, pr := range reservedPortRanges {
		pr, err := ParsePortRange(pr)
		if err != nil {
			return err
		}
		prs.ReservedPortRanges = append(prs.ReservedPortRanges, pr)
	}
	return nil
}

func (prs *PortRanges) Contains(p int) bool {
	for _, pr := range prs.ReservedPortRanges {
		if pr.Contains(p) {
			return false
		}
	}
	return prs.PortRange.Contains(p)
}

func (prs *PortRanges) GetMinAndMaxPort() (int, int) {
	return prs.PortRange.Base, prs.PortRange.Base + prs.PortRange.Size - 1
}

func ParsePortRanges(portRange, reservedPortRanges string) (*PortRanges, error) {
	prs := &PortRanges{}
	err := prs.Set(portRange)
	if err != nil {
		return nil, err
	}
	if reservedPortRanges == "" {
		return prs, nil
	}
	err = prs.SetReservedPortRanges(reservedPortRanges)
	if err != nil {
		return nil, err
	}
	return prs, nil
}
