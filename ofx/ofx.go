package ofx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/golang/glog"
)

type TransactionType string

const (
	// Common Transaction Types
	DEBIT  TransactionType = "DEBIT"
	CREDIT TransactionType = "CREDIT"
	// Uncommon Transaction Types
	INTEREST      TransactionType = "INT"
	DIVIDENT      TransactionType = "DIV"
	FEE           TransactionType = "FEE"
	SERVICECHARGE TransactionType = "SRVCHG"
	DEPOSIT       TransactionType = "DEP"
	ATM           TransactionType = "ATM"
	POS           TransactionType = "POS"
	TRANSFER      TransactionType = "XFER"
	CHECK         TransactionType = "CHECK"
	PAYMENT       TransactionType = "PAYMENT"
	CASH          TransactionType = "CASH"
	DIRECTDEPOSIT TransactionType = "DIRECTDEP"
	DIRECTDEBIT   TransactionType = "DIRECTDEBIT"
	REPEATPAYMENT TransactionType = "REPEATPMT"
	OTHER         TransactionType = "OTHER"
)

type Amount float64

func (a Amount) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	return e.EncodeElement(fmt.Sprintf("%0.2f", a), start)
}

type Transaction struct {
	Type   TransactionType `xml:"TRNTYPE"`
	Posted string          `xml:"DTPOSTED"`
	Amount Amount          `xml:"TRNAMT"`
	ID     string          `xml:"FITID"`
	Date   string          `xml:"DTUSER,omitempty"`
	Name   string          `xml:"NAME,omitempty"`
	Payee  string          `xml:"PAYEE,omitempty"`
	Memo   string          `xml:"MEMO,omitempty"`
}

type SignOnResponse struct {
	Code           int    `xml:"STATUS>CODE"`
	Severity       string `xml:"STATUS>SEVERITY"`
	Date           string `xml:"DTSERVER"`
	Language       string `xml:"LANGUAGE"`
	Organization   string `xml:"FI>ORG"`
	OrganizationID string `xml:"FI>FID"`
	IntuitID       string `xml:"INTU.BID,omitempty"`
}

type StatementTransactionResponseSet struct {
	ID       int                  `xml:"TRNUID"`
	Code     int                  `xml:"STATUS>CODE"`
	Severity string               `xml:"STATUS>SEVERITY"`
	RS       StatementResponseSet `xml:"STMTRS"`
}

type Balance struct {
	Amount Amount `xml:"BALAMT"`
	Date   string `xml:"DTASOF"`
}

type StatementResponseSet struct {
	Currency         string        `xml:"CURDEF"`
	BankID           string        `xml:"BANKACCTFROM>BANKID"`
	AccountID        string        `xml:"BANKACCTFROM>ACCTID"`
	AccountType      string        `xml:"BANKACCTFROM>ACCTTYPE"`
	StartDate        string        `xml:"BANKTRANLIST>DTSTART"`
	EndDate          string        `xml:"BANKTRANLIST>DTEND"`
	Transactions     []Transaction `xml:"BANKTRANLIST>STMTTRN"`
	LedgerBalance    Balance       `xml:"LEDGERBAL"`
	AvailableBalance Balance       `xml:"AVAILBAL"`
}

// Document is a parsed OFX/QFX Statement.
// This does not implement the complete rfc spec yet.
type Document struct {
	XMLName  xml.Name                        `xml:"OFX"`
	Response SignOnResponse                  `xml:"SIGNONMSGSRSV1>SONRS"`
	TRS      StatementTransactionResponseSet `xml:"BANKMSGSRSV1>STMTTRNRS"`
}

func NewDocumentFromXML(filename string) (*Document, error) {
	var (
		document = &Document{} // The parsed document.

		data     []byte // Buffer to parse raw bytes from the input file.
		xmlIndex int    // Index for start of XML like data.

		tagStack   = make([]*xml.StartElement, 1000) // A stack to keep parsed tags.
		lastTagIdx = -1                              // Index for the last tag on the stack.
		endMarker  bool                              // flag to indicate an expected but missing closing tag.
		cleanXML   bytes.Buffer                      // Buffer to hold cleaned XML.

		err error
	)

	// Parse raw byte from the source file into data.
	if data, err = ioutil.ReadFile(filename); err != nil {
		return nil, err
	}
	// Detect the start of XML like data.
	if xmlIndex = bytes.Index(data, []byte("<OFX>")); xmlIndex == -1 {
		return nil, fmt.Errorf("error - invalid file, OFX tag not found")
	}
	// Start a xml decoder on the context of source data that is XML like.
	reader := bytes.NewReader(data[xmlIndex:])
	decoder := xml.NewDecoder(reader)

	// Read parsed XML tokens from the XML decoder into token and
	// re-assemble them into another buffer, while adding any missing
	// closing tags and trimming spaces/newlines.
	for {
		token, err := decoder.RawToken()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		switch t := token.(type) {
		case xml.StartElement:
			glog.Infof("case start element %s", t.Name.Local)
			// Before opening a new element, we check if there is an end marker meaning, we're expecting a
			// previous tag to be closed first. If so, we close the previous tag first and reset the end marker.
			if endMarker && lastTagIdx > 0 {
				glog.Info("end marker is set, closing previous tags.")
				writeEndTag(tagStack[lastTagIdx].Name, &cleanXML)
				lastTagIdx--
				endMarker = false
			}
			// Write the new tag to clean XML buffer and put it on the tag stack as well.
			lastTagIdx++
			tagStack[lastTagIdx] = &t
			writeStartTag(&t, &cleanXML)
		case xml.CharData:
			cleanData := escapeString(strings.TrimSpace(string([]byte(t))))
			glog.Infof("case chardata (%s) %#v", cleanData, t)
			if cleanData == "" {
				continue
			}
			glog.Infof("wrote non empty data %s %v", cleanData, t)
			if _, err = cleanXML.WriteString(cleanData); err != nil {
				return nil, err
			}
			// We set the end marker after we just write chardata to the cleaned xml buffer.
			// This implies that we are expecting an end tag right after this. This assumes
			// that a given element will not have both chardata and nested tags. Which means
			// that if we just saw chardata for the current token, there will not be any nested
			// elements and we can expect a end element next. If an end element is present in
			// the source data, the end element case next should reset the endMarker. If the
			// end element is missing from the source data, the next start element will check
			// for it and close the offending previous element before starting a new one.
			endMarker = true
		case xml.EndElement:
			glog.Infof("case end element %s", t.Name.Local)
			// Close every open tag till we match the current closing tag.
			for lastTagIdx > -1 {
				lastTag := tagStack[lastTagIdx].Name
				writeEndTag(tagStack[lastTagIdx].Name, &cleanXML)
				lastTagIdx--
				// If the end element matches the last tag on the stack, pop it off the stack
				// and reset the end marker, since we have closed that tag.
				if lastTag.Local == t.Name.Local {
					endMarker = false
					break
				}
			}
		}
	}
	glog.Infof("cleanXML: %s", cleanXML.String())
	if err = xml.Unmarshal(cleanXML.Bytes(), document); err != nil {
		return nil, err
	}
	return document, nil
}
