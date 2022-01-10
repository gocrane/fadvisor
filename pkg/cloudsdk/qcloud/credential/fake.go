package credential

import (
	"fmt"
	"sync"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
)

type FakeCred struct {
	lock              sync.Mutex
	assumedSecretId   string
	assumedSecretKey  string
	assumedToken      string
	customedSecretId  string
	customedSecretKey string
}

func NewFakeCred(secretId, secretKey string, expiredDuration time.Duration) QCloudCredential {
	return &FakeCred{
		customedSecretId:  secretId,
		customedSecretKey: secretKey,
	}
}

func (cred *FakeCred) GetQCloudAssumedCredential() (*common.Credential, error) {
	if len(cred.assumedToken) == 0 || len(cred.assumedSecretId) == 0 || len(cred.assumedSecretKey) == 0 {
		return nil, fmt.Errorf("no assumed cred")
	}
	return &common.Credential{SecretId: cred.assumedSecretId, SecretKey: cred.assumedSecretKey, Token: cred.assumedToken}, nil
}

func (cred *FakeCred) GetQCloudCustomCredential() *common.Credential {

	credential := &common.Credential{}

	credential.SecretId = cred.customedSecretId
	credential.SecretKey = cred.customedSecretKey

	return credential
}

func (cred *FakeCred) GetQCloudCredential() *common.Credential {
	cred.lock.Lock()
	defer cred.lock.Unlock()

	if cred.customedSecretId != "" && cred.customedSecretKey != "" {
		return &common.Credential{
			SecretId:  cred.customedSecretId,
			SecretKey: cred.customedSecretKey,
		}
	} else {
		return &common.Credential{SecretId: cred.assumedSecretId, SecretKey: cred.assumedSecretKey, Token: cred.assumedToken}
	}
}

func (cred *FakeCred) UpdateQCloudCustomCredential(secretId, secretKey string) *common.Credential {
	cred.lock.Lock()
	defer cred.lock.Unlock()

	cred.customedSecretId = secretId
	cred.customedSecretKey = secretKey
	return &common.Credential{SecretId: cred.customedSecretId, SecretKey: cred.customedSecretKey}
}
