package pkg

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// don't remove these!
const svcSystemName = "SyxSvcRndQt"
const svcDisplayName = "SyxSvcRndQt"
const svcDescription = "Svc to put random quotes in the Windows Event log!"
const baseURI = "https://api.quotable.io/quotes/random"

type Quote struct {
	QtContent string `json:"content"`
	QtAuthor  string `json:"author"`
}

// called by Execute()
func InitCustomCode() {

	ticker := time.NewTicker(1 * time.Hour)
	done := make(chan bool)

	for {
		select {
		case <-done:
			return
		case t := <-ticker.C:
			quote, err := GetRandomQuote()
			if err != nil {
				windowsEventlog.Error(999, err.Error())
			} else {
				windowsEventlog.Info(999, fmt.Sprintf("(%v) \n \"%s\" - %s", t, quote.QtContent, quote.QtAuthor))
			}
		}
	}

}

func GetRandomQuote() (Quote, error) {

	var quoteStr []Quote

	res, err := http.Get(baseURI)
	if err != nil {
		return Quote{}, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		dataRead, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return Quote{}, err
		}
		json.Unmarshal(dataRead, &quoteStr)
		return quoteStr[0], nil
	}

	return Quote{}, nil
}
