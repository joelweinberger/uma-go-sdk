package uma

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// LnurlpRequest is the first request in the UMA protocol.
// It is sent by the VASP that is sending the payment to find out information about the receiver.
type LnurlpRequest struct {
	// ReceiverAddress is the address of the user at VASP2 that is receiving the payment.
	ReceiverAddress string
	// Nonce is a random string that is used to prevent replay attacks.
	Nonce *string
	// Signature is the base64-encoded signature of sha256(ReceiverAddress|Nonce|Timestamp).
	Signature *string
	// IsSubjectToTravelRule indicates VASP1 is a financial institution that requires travel rule information.
	IsSubjectToTravelRule *bool
	// VaspDomain is the domain of the VASP that is sending the payment. It will be used by VASP2 to fetch the public keys of VASP1.
	VaspDomain *string
	// Timestamp is the unix timestamp of when the request was sent. Used in the signature.
	Timestamp *time.Time
	// UmaVersion is the version of the UMA protocol that VASP1 prefers to use for this transaction. For the version
	// negotiation flow, see https://static.swimlanes.io/87f5d188e080cb8e0494e46f80f2ae74.png
	UmaVersion *string
}

// AsUmaRequest returns the request as an UmaLnurlpRequest if it is a valid UMA request, otherwise it returns nil.
// This is useful for validation and avoiding nil pointer dereferences.
func (q *LnurlpRequest) AsUmaRequest() *UmaLnurlpRequest {
	if !q.IsUmaRequest() {
		return nil
	}
	return &UmaLnurlpRequest{
		LnurlpRequest:         *q,
		ReceiverAddress:       q.ReceiverAddress,
		Nonce:                 *q.Nonce,
		Signature:             *q.Signature,
		IsSubjectToTravelRule: q.IsSubjectToTravelRule != nil && *q.IsSubjectToTravelRule,
		VaspDomain:            *q.VaspDomain,
		Timestamp:             *q.Timestamp,
		UmaVersion:            *q.UmaVersion,
	}
}

// IsUmaRequest returns true if the request is a valid UMA request, otherwise, if any fields are missing, it returns false.
func (q *LnurlpRequest) IsUmaRequest() bool {
	return q.VaspDomain != nil && q.Nonce != nil && q.Signature != nil && q.Timestamp != nil && q.UmaVersion != nil
}

func (q *LnurlpRequest) EncodeToUrl() (*url.URL, error) {
	receiverAddressParts := strings.Split(q.ReceiverAddress, "@")
	if len(receiverAddressParts) != 2 {
		return nil, errors.New("invalid receiver address")
	}
	scheme := "https"
	if IsDomainLocalhost(receiverAddressParts[1]) {
		scheme = "http"
	}
	lnurlpUrl := url.URL{
		Scheme: scheme,
		Host:   receiverAddressParts[1],
		Path:   fmt.Sprintf("/.well-known/lnurlp/%s", receiverAddressParts[0]),
	}
	queryParams := lnurlpUrl.Query()
	if q.IsUmaRequest() {
		queryParams.Add("signature", *q.Signature)
		queryParams.Add("vaspDomain", *q.VaspDomain)
		queryParams.Add("nonce", *q.Nonce)
		isSubjectToTravelRule := *q.IsSubjectToTravelRule
		queryParams.Add("isSubjectToTravelRule", strconv.FormatBool(isSubjectToTravelRule))
		queryParams.Add("timestamp", strconv.FormatInt(q.Timestamp.Unix(), 10))
		queryParams.Add("umaVersion", *q.UmaVersion)
	}
	lnurlpUrl.RawQuery = queryParams.Encode()
	return &lnurlpUrl, nil
}

// UmaLnurlpRequest is the first request in the UMA protocol.
// It is sent by the VASP that is sending the payment to find out information about the receiver.
type UmaLnurlpRequest struct {
	LnurlpRequest
	// ReceiverAddress is the address of the user at VASP2 that is receiving the payment.
	ReceiverAddress string
	// Nonce is a random string that is used to prevent replay attacks.
	Nonce string
	// Signature is the base64-encoded signature of sha256(ReceiverAddress|Nonce|Timestamp).
	Signature string
	// IsSubjectToTravelRule indicates VASP1 is a financial institution that requires travel rule information.
	IsSubjectToTravelRule bool
	// VaspDomain is the domain of the VASP that is sending the payment. It will be used by VASP2 to fetch the public keys of VASP1.
	VaspDomain string
	// Timestamp is the unix timestamp of when the request was sent. Used in the signature.
	Timestamp time.Time
	// UmaVersion is the version of the UMA protocol that VASP1 prefers to use for this transaction. For the version
	// negotiation flow, see https://static.swimlanes.io/87f5d188e080cb8e0494e46f80f2ae74.png
	UmaVersion string
}

func (q *LnurlpRequest) signablePayload() ([]byte, error) {
	if q.Timestamp == nil || q.Nonce == nil {
		return nil, errors.New("timestamp and nonce are required for signing")
	}
	payloadString := strings.Join([]string{q.ReceiverAddress, *q.Nonce, strconv.FormatInt(q.Timestamp.Unix(), 10)}, "|")
	return []byte(payloadString), nil
}

// LnurlpResponse is the response to the LnurlpRequest.
// It is sent by the VASP that is receiving the payment to provide information to the sender about the receiver.
type LnurlpResponse struct {
	Tag             string `json:"tag"`
	Callback        string `json:"callback"`
	MinSendable     int64  `json:"minSendable"`
	MaxSendable     int64  `json:"maxSendable"`
	EncodedMetadata string `json:"metadata"`
	// Currencies is the list of currencies that the receiver can quote. See LUD-21. Required for UMA.
	Currencies *[]Currency `json:"currencies"`
	// RequiredPayerData the data about the payer that the sending VASP must provide in order to send a payment.
	RequiredPayerData *CounterPartyDataOptions `json:"payerData"`
	// Compliance is compliance-related data from the receiving VASP for UMA.
	Compliance *LnurlComplianceResponse `json:"compliance"`
	// UmaVersion is the version of the UMA protocol that VASP2 has chosen for this transaction based on its own support
	// and VASP1's specified preference in the LnurlpRequest. For the version negotiation flow, see
	// https://static.swimlanes.io/87f5d188e080cb8e0494e46f80f2ae74.png
	UmaVersion *string `json:"umaVersion"`
	// CommentCharsAllowed is the number of characters that the sender can include in the comment field of the pay request.
	CommentCharsAllowed *int `json:"commentAllowed"`
	// NostrPubkey is an optional nostr pubkey used for nostr zaps (NIP-57). If set, it should be a valid BIP-340 public
	// key in hex format.
	NostrPubkey *string `json:"nostrPubkey"`
	// AllowsNostr should be set to true if the receiving VASP allows nostr zaps (NIP-57).
	AllowsNostr *bool `json:"allowsNostr"`
}

// LnurlComplianceResponse is the `compliance` field  of the LnurlpResponse.
type LnurlComplianceResponse struct {
	// KycStatus indicates whether VASP2 has KYC information about the receiver.
	KycStatus KycStatus `json:"kycStatus"`
	// Signature is the base64-encoded signature of sha256(ReceiverAddress|Nonce|Timestamp).
	Signature string `json:"signature"`
	// Nonce is a random string that is used to prevent replay attacks.
	Nonce string `json:"signatureNonce"`
	// Timestamp is the unix timestamp of when the request was sent. Used in the signature.
	Timestamp int64 `json:"signatureTimestamp"`
	// IsSubjectToTravelRule indicates whether VASP2 is a financial institution that requires travel rule information.
	IsSubjectToTravelRule bool `json:"isSubjectToTravelRule"`
	// ReceiverIdentifier is the identifier of the receiver at VASP2.
	ReceiverIdentifier string `json:"receiverIdentifier"`
}

func (r *LnurlpResponse) IsUmaResponse() bool {
	return r.Compliance != nil && r.UmaVersion != nil && r.Currencies != nil && r.RequiredPayerData != nil
}

func (r *LnurlpResponse) AsUmaResponse() *UmaLnurlpResponse {
	if !r.IsUmaResponse() {
		return nil
	}
	return &UmaLnurlpResponse{
		LnurlpResponse:      *r,
		Currencies:          *r.Currencies,
		RequiredPayerData:   *r.RequiredPayerData,
		Compliance:          *r.Compliance,
		UmaVersion:          *r.UmaVersion,
		CommentCharsAllowed: r.CommentCharsAllowed,
		NostrPubkey:         r.NostrPubkey,
		AllowsNostr:         r.AllowsNostr,
	}
}

// UmaLnurlpResponse is the UMA response to the LnurlpRequest.
// It is sent by the VASP that is receiving the payment to provide information to the sender about the receiver.
type UmaLnurlpResponse struct {
	LnurlpResponse
	// Currencies is the list of currencies that the receiver can quote. See LUD-21. Required for UMA.
	Currencies []Currency `json:"currencies"`
	// RequiredPayerData the data about the payer that the sending VASP must provide in order to send a payment.
	RequiredPayerData CounterPartyDataOptions `json:"payerData"`
	// Compliance is compliance-related data from the receiving VASP for UMA.
	Compliance LnurlComplianceResponse `json:"compliance"`
	// UmaVersion is the version of the UMA protocol that VASP2 has chosen for this transaction based on its own support
	// and VASP1's specified preference in the LnurlpRequest. For the version negotiation flow, see
	// https://static.swimlanes.io/87f5d188e080cb8e0494e46f80f2ae74.png
	UmaVersion string `json:"umaVersion"`
	// CommentCharsAllowed is the number of characters that the sender can include in the comment field of the pay request.
	CommentCharsAllowed *int `json:"commentAllowed"`
	// NostrPubkey is an optional nostr pubkey used for nostr zaps (NIP-57). If set, it should be a valid BIP-340 public
	// key in hex format.
	NostrPubkey *string `json:"nostrPubkey"`
	// AllowsNostr should be set to true if the receiving VASP allows nostr zaps (NIP-57).
	AllowsNostr *bool `json:"allowsNostr"`
}

func (r *UmaLnurlpResponse) signablePayload() []byte {
	payloadString := strings.Join([]string{
		r.Compliance.ReceiverIdentifier,
		r.Compliance.Nonce,
		strconv.FormatInt(r.Compliance.Timestamp, 10),
	}, "|")
	return []byte(payloadString)
}

// PayRequest is the request sent by the sender to the receiver to retrieve an invoice.
type PayRequest struct {
	// SendingAmountCurrencyCode is the currency code of the `amount` field. `nil` indicates that `amount` is in
	// millisatoshis as in LNURL without LUD-21. If this is not `nil`, then `amount` is in the smallest unit of the
	// specified currency (e.g. cents for USD). This currency code can be any currency which the receiver can quote.
	// However, there are two most common scenarios for UMA:
	//
	// 1. If the sender wants the receiver wants to receive a specific amount in their receiving
	// currency, then this field should be the same as `receiving_currency_code`. This is useful
	// for cases where the sender wants to ensure that the receiver receives a specific amount
	// in that destination currency, regardless of the exchange rate, for example, when paying
	// for some goods or services in a foreign currency.
	//
	// 2. If the sender has a specific amount in their own currency that they would like to send,
	// then this field should be left as `None` to indicate that the amount is in millisatoshis.
	// This will lock the sent amount on the sender side, and the receiver will receive the
	// equivalent amount in their receiving currency. NOTE: In this scenario, the sending VASP
	// *should not* pass the sending currency code here, as it is not relevant to the receiver.
	// Rather, by specifying an invoice amount in msats, the sending VASP can ensure that their
	// user will be sending a fixed amount, regardless of the exchange rate on the receiving side.
	SendingAmountCurrencyCode *string `json:"sendingAmountCurrencyCode"`
	// ReceivingCurrencyCode is the ISO 3-digit currency code that the receiver will receive for this payment. Defaults
	// to amount being specified in msats if this is not provided.
	ReceivingCurrencyCode *string `json:"convert"`
	// Amount is the amount that the receiver will receive for this payment in the smallest unit of the specified
	// currency (i.e. cents for USD) if `SendingAmountCurrencyCode` is not `nil`. Otherwise, it is the amount in
	// millisatoshis.
	Amount int64 `json:"amount"`
	// PayerData is the data that the sender will send to the receiver to identify themselves. Required for UMA, as is
	// the `compliance` field in the `payerData` object.
	PayerData *PayerData `json:"payerData"`
	// RequestedPayeeData is the data that the sender is requesting about the payee.
	RequestedPayeeData *CounterPartyDataOptions `json:"payeeData"`
	// Comment is a comment that the sender would like to include with the payment. This can only be included
	// if the receiver included the `commentAllowed` field in the lnurlp response. The length of
	// the comment must be less than or equal to the value of `commentAllowed`.
	Comment *string `json:"comment"`
}

// IsUmaRequest returns true if the request is a valid UMA request, otherwise, if any fields are missing, it returns false.
func (p *PayRequest) IsUmaRequest() bool {
	if p.PayerData == nil {
		return false
	}

	compliance, err := p.PayerData.Compliance()
	if err != nil {
		return false
	}

	return compliance != nil && p.PayerData.Identifier() != nil
}

func (p *PayRequest) MarshalJSON() ([]byte, error) {
	amount := strconv.FormatInt(p.Amount, 10)
	if p.SendingAmountCurrencyCode != nil {
		amount = fmt.Sprintf("%s.%s", amount, *p.SendingAmountCurrencyCode)
	}
	var payerDataJson []byte
	if p.PayerData != nil {
		var err error
		payerDataJson, err = json.Marshal(p.PayerData)
		if err != nil {
			return nil, err
		}
	}
	reqStr := fmt.Sprintf(`{
		"amount": "%s"`, amount)
	if p.ReceivingCurrencyCode != nil {
		reqStr += fmt.Sprintf(`,
		"convert": "%s"`, *p.ReceivingCurrencyCode)
	}
	if p.PayerData != nil {
		reqStr += fmt.Sprintf(`,
		"payerData": %s`, payerDataJson)
	}
	if p.RequestedPayeeData != nil {
		payeeDataJson, err := json.Marshal(p.RequestedPayeeData)
		if err != nil {
			return nil, err
		}
		reqStr += fmt.Sprintf(`,
		"payeeData": %s`, payeeDataJson)
	}
	if p.Comment != nil {
		reqStr += fmt.Sprintf(`,
		"comment": "%s"`, *p.Comment)
	}
	reqStr += "}"
	return []byte(reqStr), nil
}

func (p *PayRequest) UnmarshalJSON(data []byte) error {
	var rawReq map[string]interface{}
	err := json.Unmarshal(data, &rawReq)
	if err != nil {
		return err
	}
	convert, ok := rawReq["convert"].(string)
	if ok {
		p.ReceivingCurrencyCode = &convert
	}
	amount, ok := rawReq["amount"].(string)
	if !ok {
		return errors.New("missing or invalid amount field")
	}
	amountParts := strings.Split(amount, ".")
	if len(amountParts) > 2 {
		return errors.New("invalid amount field")
	}
	p.Amount, err = strconv.ParseInt(amountParts[0], 10, 64)
	if err != nil {
		return err
	}
	if len(amountParts) == 2 && len(amountParts[1]) > 0 {
		p.SendingAmountCurrencyCode = &amountParts[1]
	}
	payerDataJson, ok := rawReq["payerData"].(map[string]interface{})
	if ok {
		payerDataJsonBytes, err := json.Marshal(payerDataJson)
		if err != nil {
			return err
		}
		var payerData PayerData
		err = json.Unmarshal(payerDataJsonBytes, &payerData)
		if err != nil {
			return err
		}
		p.PayerData = &payerData
	}
	payeeDataJson, ok := rawReq["payeeData"].(map[string]interface{})
	if ok {
		payeeDataJsonBytes, err := json.Marshal(payeeDataJson)
		if err != nil {
			return err
		}
		var payeeData CounterPartyDataOptions
		err = json.Unmarshal(payeeDataJsonBytes, &payeeData)
		if err != nil {
			return err
		}
		p.RequestedPayeeData = &payeeData
	}
	comment, ok := rawReq["comment"].(string)
	if ok {
		p.Comment = &comment
	}
	return nil
}

func (p *PayRequest) Encode() ([]byte, error) {
	return json.Marshal(p)
}

func (p *PayRequest) EncodeAsUrlParams() (*url.Values, error) {
	jsonBytes, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	jsonMap := make(map[string]interface{})
	err = json.Unmarshal(jsonBytes, &jsonMap)
	if err != nil {
		return nil, err
	}
	payReqParams := url.Values{}
	for key, value := range jsonMap {
		valueString, ok := value.(string)
		if ok {
			payReqParams.Add(key, valueString)
		} else {
			valueBytes, err := json.Marshal(value)
			if err != nil {
				return nil, err
			}
			payReqParams.Add(key, string(valueBytes))
		}
	}
	return &payReqParams, nil
}

func (p *PayRequest) signablePayload() ([]byte, error) {
	if p.PayerData == nil {
		return nil, errors.New("payer data is missing")
	}
	senderAddress := p.PayerData.Identifier()
	if senderAddress == nil || *senderAddress == "" {
		return nil, errors.New("payer data identifier is missing")
	}
	complianceData, err := p.PayerData.Compliance()
	if err != nil {
		return nil, err
	}
	if complianceData == nil {
		return nil, errors.New("compliance payer data is missing")
	}
	signatureNonce := complianceData.SignatureNonce
	signatureTimestamp := complianceData.SignatureTimestamp
	payloadString := strings.Join([]string{
		*senderAddress,
		signatureNonce,
		strconv.FormatInt(signatureTimestamp, 10),
	}, "|")
	return []byte(payloadString), nil
}

// PayReqResponse is the response sent by the receiver to the sender to provide an invoice.
type PayReqResponse struct {
	// EncodedInvoice is the BOLT11 invoice that the sender will pay.
	EncodedInvoice string `json:"pr"`
	// Routes is usually just an empty list from legacy LNURL, which was replaced by route hints in the BOLT11 invoice.
	Routes []Route `json:"routes"`
	// PaymentInfo is information about the payment that the receiver will receive. Includes Final currency-related
	// information for the payment. Required for UMA.
	PaymentInfo *PayReqResponsePaymentInfo `json:"paymentInfo"`
	// PayeeData The data about the receiver that the sending VASP requested in the payreq request.
	// Required for UMA.
	PayeeData *PayeeData `json:"payeeData"`
	// Disposable This field may be used by a WALLET to decide whether the initial LNURL link will  be stored locally
	// for later reuse or erased. If disposable is null, it should be interpreted as true, so if SERVICE intends its
	// LNURL links to be stored it must return `disposable: false`. UMA should never return `disposable: false` due to
	// signature nonce checks, etc. See LUD-11.
	Disposable *bool `json:"disposable"`
	// SuccessAction defines a struct which can be stored and shown to the user on payment success. See LUD-09.
	SuccessAction *map[string]string `json:"successAction"`
}

func (p *PayReqResponse) IsUmaResponse() bool {
	if p.PaymentInfo == nil || p.PayeeData == nil {
		return false
	}
	compliance, err := p.PayeeData.Compliance()
	if err != nil {
		return false
	}
	return compliance != nil
}

type Route struct {
	Pubkey string `json:"pubkey"`
	Path   []struct {
		Pubkey   string `json:"pubkey"`
		Fee      int64  `json:"fee"`
		Msatoshi int64  `json:"msatoshi"`
		Channel  string `json:"channel"`
	} `json:"path"`
}

type PayReqResponsePaymentInfo struct {
	// Amount is the amount that the receiver will receive in the receiving currency not including fees. The amount is
	//    specified in the smallest unit of the currency (eg. cents for USD).
	Amount int64 `json:"amount"`
	// CurrencyCode is the currency code that the receiver will receive for this payment.
	CurrencyCode string `json:"currencyCode"`
	// Multiplier is the conversion rate. It is the number of millisatoshis that the receiver will receive for 1 unit of
	//    the specified currency. In this context, this is just for convenience. The conversion rate is also baked into
	//    the invoice amount itself.
	//    `invoice amount = Amount * Multiplier + ExchangeFeesMillisatoshi`
	Multiplier float64 `json:"multiplier"`
	// Decimals is the number of digits after the decimal point for the receiving currency. For example, in USD, by
	// convention, there are 2 digits for cents - $5.95. In this case, `Decimals` would be 2. This should align with the
	// currency's `Decimals` field in the LNURLP response. It is included here for convenience. See
	// [UMAD-04](/uma-04-local-currency.md) for details, edge cases, and examples.
	Decimals int `json:"decimals"`
	// ExchangeFeesMillisatoshi is the fees charged (in millisats) by the receiving VASP for this transaction. This is
	// separate from the Multiplier.
	ExchangeFeesMillisatoshi int64 `json:"fee"`
}

func (c *CompliancePayeeData) signablePayload(payerIdentifier string, payeeIdentifier string) ([]byte, error) {
	if c == nil {
		return nil, errors.New("compliance data is missing")
	}
	payloadString := strings.Join([]string{
		payerIdentifier,
		payeeIdentifier,
		c.SignatureNonce,
		strconv.FormatInt(c.SignatureTimestamp, 10),
	}, "|")
	return []byte(payloadString), nil
}

// PubKeyResponse is sent from a VASP to another VASP to provide its public keys.
// It is the response to GET requests at `/.well-known/lnurlpubkey`.
type PubKeyResponse struct {
	// SigningPubKeyHex is used to verify signatures from a VASP. Hex-encoded byte array.
	SigningPubKeyHex string `json:"signingPubKey"`
	// EncryptionPubKeyHex is used to encrypt TR info sent to a VASP. Hex-encoded byte array.
	EncryptionPubKeyHex string `json:"encryptionPubKey"`
	// ExpirationTimestamp [Optional] Seconds since epoch at which these pub keys must be refreshed.
	// They can be safely cached until this expiration (or forever if null).
	ExpirationTimestamp *int64 `json:"expirationTimestamp"`
}

func (r *PubKeyResponse) SigningPubKey() ([]byte, error) {
	return hex.DecodeString(r.SigningPubKeyHex)
}

func (r *PubKeyResponse) EncryptionPubKey() ([]byte, error) {
	return hex.DecodeString(r.EncryptionPubKeyHex)
}

// UtxoWithAmount is a pair of utxo and amount transferred over that corresponding channel.
// It can be used to register payment for KYT.
type UtxoWithAmount struct {
	// Utxo The utxo of the channel over which the payment went through in the format of <transaction_hash>:<output_index>.
	Utxo string `json:"utxo"`

	// Amount The amount of funds transferred in the payment in mSats.
	Amount int64 `json:"amountMsats"`
}

// PostTransactionCallback is sent between VASPs after the payment is complete.
type PostTransactionCallback struct {
	// Utxos is a list of utxo/amounts corresponding to the VASPs channels.
	Utxos []UtxoWithAmount `json:"utxos"`
	// VaspDomain is the domain of the VASP that is sending the callback.
	// It will be used by the VASP to fetch the public keys of its counterparty.
	VaspDomain string `json:"vaspDomain"`
	// Signature is the base64-encoded signature of sha256(Nonce|Timestamp).
	Signature string `json:"signature"`
	// Nonce is a random string that is used to prevent replay attacks.
	Nonce string `json:"signatureNonce"`
	// Timestamp is the unix timestamp of when the request was sent. Used in the signature.
	Timestamp int64 `json:"signatureTimestamp"`
}

func (c *PostTransactionCallback) signablePayload() []byte {
	payloadString := strings.Join([]string{
		c.Nonce,
		strconv.FormatInt(c.Timestamp, 10),
	}, "|")
	return []byte(payloadString)
}
