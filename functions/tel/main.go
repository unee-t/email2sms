package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/endpoints"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/jhillyerd/enmime"
	"github.com/pkg/errors"
	"github.com/unee-t/env"
)

func LambdaHandler(ctx context.Context, payload events.SNSEvent) (err error) {

	j, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "unable to marshal")
	}

	log.Infof("JSON payload %s", string(j))

	var email events.SimpleEmailService

	err = json.Unmarshal([]byte(payload.Records[0].SNS.Message), &email)
	if err != nil {
		return errors.Wrap(err, "bad JSON")
	}

	h, err := New()
	if err != nil {
		return errors.Wrap(err, "error setting configuration")
	}

	err = h.inbox(email)
	if err != nil {
		return errors.Wrap(err, "could not inbox")
	}
	return

}

func main() {
	lambda.Start(LambdaHandler)
}

type handler struct {
	Env env.Env // Env.cfg for the AWS cfg
}

func New() (h handler, err error) {
	cfg, err := external.LoadDefaultAWSConfig(external.WithSharedConfigProfile("uneet-dev"))
	if err != nil {
		return
	}
	cfg.Region = endpoints.ApSoutheast1RegionID
	e, err := env.New(cfg)
	if err != nil {
		return
	}
	h.Env = e
	return
}

func (h handler) inbox(email events.SimpleEmailService) (err error) {
	svc := s3.New(h.Env.Cfg)

	rawMessage := fmt.Sprintf("tel/%s", email.Mail.MessageID)

	input := &s3.GetObjectInput{
		Bucket: aws.String(h.Env.Bucket("email")), // Goto env
		Key:    aws.String(rawMessage),
	}

	//fmt.Println(input)

	req := svc.GetObjectRequest(input)
	original, err := req.Send()
	if err != nil {
		return
	}
	// fmt.Println(original.Body)

	// Update read permissions
	envelope, err := enmime.ReadEnvelope(original.Body)
	aclputparams := &s3.PutObjectAclInput{
		Bucket: aws.String(h.Env.Bucket("email")),
		Key:    aws.String(rawMessage),
		ACL:    s3.ObjectCannedACLPublicRead,
	}

	s3aclreq := svc.PutObjectAclRequest(aclputparams)
	_, err = s3aclreq.Send()
	if err != nil {
		return
	}

	textPartKey := time.Now().Format("2006-01-02") + "/" + email.Mail.MessageID + "/text"

	putparams := &s3.PutObjectInput{
		Bucket:      aws.String(h.Env.Bucket("email")),
		Body:        bytes.NewReader([]byte(envelope.Text)),
		Key:         aws.String(textPartKey),
		ContentType: aws.String("text/plain; charset=UTF-8"),
		ACL:         s3.ObjectCannedACLPublicRead,
	}

	s3req := svc.PutObjectRequest(putparams)
	_, err = s3req.Send()
	if err != nil {
		return
	}

	htmlPart := time.Now().Format("2006-01-02") + "/" + email.Mail.MessageID + "/html"

	putparams = &s3.PutObjectInput{
		Bucket:      aws.String(h.Env.Bucket("email")),
		Body:        bytes.NewReader([]byte(envelope.HTML)),
		Key:         aws.String(htmlPart),
		ContentType: aws.String("text/html; charset=UTF-8"),
		ACL:         s3.ObjectCannedACLPublicRead,
	}

	s3req = svc.PutObjectRequest(putparams)
	_, err = s3req.Send()
	if err != nil {
		return
	}

	log.WithFields(
		log.Fields{
			"to":   email.Mail.CommonHeaders.To,
			"orig": fmt.Sprintf("https://s3-ap-southeast-1.amazonaws.com/%s/tel/%s", h.Env.Bucket("email"), email.Mail.MessageID),
			"text": fmt.Sprintf("https://s3-ap-southeast-1.amazonaws.com/%s/%s", h.Env.Bucket("email"), textPartKey),
			"html": fmt.Sprintf("https://s3-ap-southeast-1.amazonaws.com/%s/%s", h.Env.Bucket("email"), htmlPart),
		}).Infof("%#v", envelope)

	toNumber, err := parseTo(email.Mail.CommonHeaders.To[0])
	if err != nil {
		log.WithError(err).Error("bad to")
		return err
	}

	if toNumber == "" {
		log.Error("empty toNumber")
		return
	}

	snssvc := sns.New(h.Env.Cfg)
	snsreq := snssvc.PublishRequest(&sns.PublishInput{
		Message:     aws.String(fmt.Sprintf("https://s3-ap-southeast-1.amazonaws.com/%s/%s", h.Env.Bucket("email"), textPartKey)),
		PhoneNumber: aws.String(toNumber),
	})

	_, err = snsreq.Send()

	return
}

func parseTo(toAddress string) (string, error) {
	log.Infof("Checking reply address is valid: %s", toAddress)

	e, err := mail.ParseAddress(toAddress)
	if err != nil {
		return "", err
	}

	if !strings.HasPrefix(e.Address, "tel+") {
		return "", fmt.Errorf("missing tel+ prefix")
	}

	log.WithField("toAddress", toAddress).Info("parsing out number")

	telParts := strings.Split(e.Address, "+")
	if len(telParts) != 2 {
		return "", fmt.Errorf("Not in tel+digits@ structure")
	}
	log.Info(telParts[1])
	numParts := strings.Split(telParts[1], "@")
	if len(numParts) != 2 {
		return "", fmt.Errorf("Not in tel+digits@ structure")
	}
	log.Info(numParts[0])

	return "+" + numParts[0], err
}
