package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/getsentry/sentry-go"
)

const AWSRegion = "eu-west-1"

var (
	awsKey       string
	awsSecret    string
	hostedZoneId string
	recordName   string
)

func init() {
	flag.StringVar(&awsKey, "key", "", "*AWS Access Key ID")
	flag.StringVar(&awsSecret, "secret", "", "*AWS Secret Key")
	flag.StringVar(&hostedZoneId, "hosted-zone-id", "", "*AWS Hosted Zone ID")
	flag.StringVar(&recordName, "record-name", "", "*Domain name")
	sentryDsn := flag.String("sentry-dsn", "", "*SentryDSN")
	whatsMyIP := flag.Bool("whats-my-ip", false, "Check my real IP address and exit")
	registeredIP := flag.Bool("registered-ip", false, "Check the IP address for record-name and exit")
	flag.Parse()

	if *sentryDsn == "" {
		log.Fatalf("SentryDSN must be specified")
	}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              *sentryDsn,
		TracesSampleRate: 1.0,
	})
	if err != nil {
		log.Fatalf("sentry.Init: %s", err)
	}

	if awsKey == "" || awsSecret == "" || hostedZoneId == "" || recordName == "" {
		sentry.CaptureMessage("You must provide a AWS Access Key ID, Secret Key, Hosted Zone ID and Record Name")
		os.Exit(1)
	}

	if *whatsMyIP {
		fmt.Printf("My current IP address is %s\n", publicIP())
		os.Exit(0)
	}

	if *registeredIP {
		fmt.Printf("Registered IP address is %s\n", registeredIp())
		os.Exit(0)
	}
}

func main() {
	defer sentry.Flush(3 * time.Second)

	pubIP := publicIP()
	if pubIP == registeredIp() {
		log.Println("IP is already updated")
		os.Exit(0)
	}

	_, err := route53Client().ChangeResourceRecordSets(context.TODO(), &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(hostedZoneId),
		ChangeBatch: &types.ChangeBatch{
			Changes: []types.Change{
				{
					Action: types.ChangeActionUpsert,
					ResourceRecordSet: &types.ResourceRecordSet{
						Name: aws.String(recordName),
						Type: types.RRTypeA,
						TTL:  aws.Int64(3600),
						ResourceRecords: []types.ResourceRecord{
							{Value: aws.String(pubIP)},
						},
					},
				},
			},
		},
	})
	if err != nil {
		sentry.CaptureException(err)
	}
}

func route53Client() *route53.Client {
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithRegion(AWSRegion),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(awsKey, awsSecret, "")),
	)
	if err != nil {
		sentry.CaptureException(err)
	}

	return route53.NewFromConfig(cfg)
}

func registeredIp() string {
	out, err := route53Client().ListResourceRecordSets(context.TODO(), &route53.ListResourceRecordSetsInput{
		HostedZoneId:    aws.String(hostedZoneId),
		StartRecordName: aws.String(recordName),
	})
	if err != nil {
		sentry.CaptureException(err)
	}

	return aws.ToString(out.ResourceRecordSets[0].ResourceRecords[0].Value)
}

func publicIP() string {
	resp, err := http.Get("http://checkip.amazonaws.com/")
	if err != nil || resp.StatusCode > 299 {
		sentry.CaptureException(err)
	}

	body, _ := io.ReadAll(resp.Body)
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			sentry.CaptureException(err)
		}
	}(resp.Body)

	return strings.TrimSpace(string(body))
}
