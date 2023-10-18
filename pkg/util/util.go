package util

import (
	"fmt"
	"net/url"
	"strings"
)

func ConvertStringToAddresses(ips string) ([]string, error) {
	arr := strings.Split(ips, ",")

	var returned []string
	for _, i := range arr {
		if _, err := url.Parse(i); err != nil {
			return nil, fmt.Errorf("error to parse ip: %s", i)
		}
		returned = append(returned, i)
	}

	return returned, nil
}
