package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"secrets-init/pkg/secrets"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/secretsmanager/secretsmanageriface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/pkg/errors"
)

// SSMEntry is a key,value pair from SSM
type SSMEntry struct {
	name  string
	value string
}

// SecretsProvider AWS secrets provider
type SecretsProvider struct {
	session *session.Session
	sm      secretsmanageriface.SecretsManagerAPI
	ssm     ssmiface.SSMAPI
}

// NewAwsSecretsProvider init AWS Secrets Provider
func NewAwsSecretsProvider() (secrets.Provider, error) {
	var err error
	sp := SecretsProvider{}
	// create AWS session
	sp.session, err = session.NewSessionWithOptions(session.Options{SharedConfigState: session.SharedConfigEnable})
	if err != nil {
		return nil, err
	}
	// init AWS Secrets Manager client
	sp.sm = secretsmanager.New(sp.session)
	// init AWS SSM client
	sp.ssm = ssm.New(sp.session)
	return &sp, nil
}

// ResolveSecrets replaces all passed variables values prefixed with 'aws:aws:secretsmanager' and 'arn:aws:ssm:REGION:ACCOUNT:parameter'
// by corresponding secrets from AWS Secret Manager and AWS Parameter Store
func (sp *SecretsProvider) ResolveSecrets(ctx context.Context, vars []string) ([]string, error) {
	var envs []string

	for _, env := range vars {
		kv := strings.Split(env, "=")
		_, value := kv[0], kv[1]
		if strings.HasPrefix(value, "arn:aws:secretsmanager") {
			// get secret value
			secret, err := sp.sm.GetSecretValue(&secretsmanager.GetSecretValueInput{SecretId: &value})
			if err != nil {
				return envs, errors.Wrap(err, "failed to get secret from AWS Secrets Manager")
			}

			/*
				{
					ARN: "arn:aws:secretsmanager:us-east-1:12345678901:secret:webserver/truck-J69aL7",
					CreatedDate: 2021-02-26 19:24:56 +0000 UTC,
					Name: "webserver/truck",
					SecretString: "{\"truck\":\"chevy\"}",
					VersionId: "7b3a6445-4278-4691-bd5e-0fcc2a87b297",
					VersionStages: ["AWSCURRENT"]
				}
			*/
			var entry map[string]string
			err = json.Unmarshal([]byte(*secret.SecretString), &entry)
			if err != nil {
				return envs, errors.Wrap(err, "failed to unmarshal json from AWS Secrets Manager")
			}

			for k, v := range entry {
				env = strings.ToUpper(k) + "=" + v
				envs = append(envs, env)
			}

		} else if strings.HasPrefix(value, "arn:aws:ssm") && strings.Contains(value, ":parameter/") {
			tokens := strings.Split(value, ":")
			// valid parameter ARN arn:aws:ssm:REGION:ACCOUNT:parameter/PATH
			// or arn:aws:ssm:REGION:ACCOUNT:parameter/PATH:VERSION
			if len(tokens) == 6 || len(tokens) == 7 {
				// get SSM parameter name (path)
				paramName := strings.TrimPrefix(tokens[5], "parameter")

				if len(tokens) == 7 {
					paramName = strings.Join([]string{paramName, tokens[6]}, ":")
				}

				// get AWS SSM API
				withDecryption := true
				param, err := sp.ssm.GetParameter(&ssm.GetParameterInput{
					Name:           &paramName,
					WithDecryption: &withDecryption,
				})
				if err != nil {
					return vars, errors.Wrap(err, "failed to get secret from AWS Parameters Store")
				}
				name := *param.Parameter.Name
				value := *param.Parameter.Value

				if strings.Contains(value, " ") {
					value = fmt.Sprintf("\"%s\"", value)
				}
				env = strings.ToUpper(name[1:]) + "=" + value
				envs = append(envs, env)
			}
		}
	}

	return envs, nil
}
