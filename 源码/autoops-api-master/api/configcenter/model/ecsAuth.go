package model

import "dodevops-api/common/util"

const maskedSecretValue = "******"

type EcsAuth struct {
	ID         uint       `gorm:"column:id;primaryKey;not null" json:"id"`
	Name       string     `gorm:"column:name;varchar(64);not null" json:"name"`
	Type       int        `gorm:"column:type;not null" json:"type"`
	Username   string     `gorm:"column:username;varchar(64)" json:"username"`
	Password   string     `gorm:"column:password;type:text" json:"password"`
	PublicKey  string     `gorm:"column:public_key;type:text" json:"publicKey"`
	Port       int        `gorm:"column:port;default:22" json:"port"`
	CreateTime util.HTime `gorm:"column:create_time;not null" json:"createTime"`
	UpdateTime util.HTime `gorm:"column:update_time" json:"updateTime"`
	Remark     string     `gorm:"column:remark;varchar(500)" json:"remark"`

	BastionHost       string `gorm:"column:bastion_host;varchar(64)" json:"bastionHost"`
	BastionPort       int    `gorm:"column:bastion_port;default:22" json:"bastionPort"`
	BastionUsername   string `gorm:"column:bastion_username;varchar(64)" json:"bastionUsername"`
	BastionAuthType   int    `gorm:"column:bastion_auth_type" json:"bastionAuthType"`
	BastionPassword   string `gorm:"column:bastion_password;type:text" json:"bastionPassword"`
	BastionPrivateKey string `gorm:"column:bastion_private_key;type:text" json:"bastionPrivateKey"`
	TargetAuthType    int    `gorm:"column:target_auth_type" json:"targetAuthType"`
	TargetPassword    string `gorm:"column:target_password;type:text" json:"targetPassword"`
	TargetPrivateKey  string `gorm:"column:target_private_key;type:text" json:"targetPrivateKey"`
}

func (EcsAuth) TableName() string {
	return "config_ecsauth"
}

func (a *EcsAuth) EncryptSecrets() error {
	var err error

	a.Password, err = encryptEcsSecretIfNeeded(a.Password)
	if err != nil {
		return err
	}
	a.PublicKey, err = encryptEcsSecretIfNeeded(a.PublicKey)
	if err != nil {
		return err
	}
	a.BastionPassword, err = encryptEcsSecretIfNeeded(a.BastionPassword)
	if err != nil {
		return err
	}
	a.BastionPrivateKey, err = encryptEcsSecretIfNeeded(a.BastionPrivateKey)
	if err != nil {
		return err
	}
	a.TargetPassword, err = encryptEcsSecretIfNeeded(a.TargetPassword)
	if err != nil {
		return err
	}
	a.TargetPrivateKey, err = encryptEcsSecretIfNeeded(a.TargetPrivateKey)
	if err != nil {
		return err
	}

	return nil
}

func (a *EcsAuth) DecryptSecrets() error {
	var err error

	a.Password, err = decryptEcsSecretIfPossible(a.Password)
	if err != nil {
		return err
	}
	a.PublicKey, err = decryptEcsSecretIfPossible(a.PublicKey)
	if err != nil {
		return err
	}
	a.BastionPassword, err = decryptEcsSecretIfPossible(a.BastionPassword)
	if err != nil {
		return err
	}
	a.BastionPrivateKey, err = decryptEcsSecretIfPossible(a.BastionPrivateKey)
	if err != nil {
		return err
	}
	a.TargetPassword, err = decryptEcsSecretIfPossible(a.TargetPassword)
	if err != nil {
		return err
	}
	a.TargetPrivateKey, err = decryptEcsSecretIfPossible(a.TargetPrivateKey)
	if err != nil {
		return err
	}

	return nil
}

func (a *EcsAuth) MaskSecrets() {
	a.Password = maskEcsSecret(a.Password)
	a.PublicKey = maskEcsSecret(a.PublicKey)
	a.BastionPassword = maskEcsSecret(a.BastionPassword)
	a.BastionPrivateKey = maskEcsSecret(a.BastionPrivateKey)
	a.TargetPassword = maskEcsSecret(a.TargetPassword)
	a.TargetPrivateKey = maskEcsSecret(a.TargetPrivateKey)
}

func (a EcsAuth) HasPassword() bool {
	return a.Password != ""
}

func (a EcsAuth) HasPublicKey() bool {
	return a.PublicKey != ""
}

func (a EcsAuth) HasBastionPassword() bool {
	return a.BastionPassword != ""
}

func (a EcsAuth) HasBastionPrivateKey() bool {
	return a.BastionPrivateKey != ""
}

func (a EcsAuth) HasTargetPassword() bool {
	return a.TargetPassword != ""
}

func (a EcsAuth) HasTargetPrivateKey() bool {
	return a.TargetPrivateKey != ""
}

func encryptEcsSecretIfNeeded(value string) (string, error) {
	if value == "" {
		return "", nil
	}

	if _, err := util.AESDecrypt(value); err == nil {
		return value, nil
	}

	return util.AESEncrypt(value)
}

func decryptEcsSecretIfPossible(value string) (string, error) {
	if value == "" {
		return "", nil
	}

	decrypted, err := util.AESDecrypt(value)
	if err != nil {
		return value, nil
	}

	return decrypted, nil
}

func maskEcsSecret(value string) string {
	if value == "" {
		return ""
	}
	return maskedSecretValue
}

func IsMaskedSecretValue(value string) bool {
	return value == maskedSecretValue
}

type CreateEcsPasswordAuthDto struct {
	Name      string `validate:"required" json:"name"`
	Type      int    `validate:"required" json:"type"`
	Username  string `validate:"required" json:"username"`
	Password  string `json:"password"`
	PublicKey string `json:"publicKey"`
	Port      int    `validate:"required" json:"port"`
	Remark    string `json:"remark"`

	BastionHost       string `json:"bastionHost"`
	BastionPort       int    `json:"bastionPort"`
	BastionUsername   string `json:"bastionUsername"`
	BastionAuthType   int    `json:"bastionAuthType"`
	BastionPassword   string `json:"bastionPassword"`
	BastionPrivateKey string `json:"bastionPrivateKey"`
	TargetAuthType    int    `json:"targetAuthType"`
	TargetPassword    string `json:"targetPassword"`
	TargetPrivateKey  string `json:"targetPrivateKey"`
}

type CreateEcsKeyAuthDto struct {
	Name      string `validate:"required"`
	Type      int    `validate:"required"`
	PublicKey string `validate:"required"`
	Username  string `validate:"required"`
	Port      int    `validate:"required"`
	Remark    string
}

type CreateEcsPublicKeyAuthDto struct {
	Name     string `validate:"required"`
	Type     int    `validate:"required"`
	Username string `validate:"required"`
	Port     int    `validate:"required"`
	Remark   string
}

type CreateEcsBastionAuthDto struct {
	Name     string `validate:"required" json:"name"`
	Type     int    `validate:"required" json:"type"`
	Username string `validate:"required" json:"username"`
	Port     int    `validate:"required" json:"port"`
	Remark   string `json:"remark"`

	BastionHost       string `validate:"required" json:"bastionHost"`
	BastionPort       int    `validate:"required" json:"bastionPort"`
	BastionUsername   string `validate:"required" json:"bastionUsername"`
	BastionAuthType   int    `validate:"required" json:"bastionAuthType"`
	BastionPassword   string `json:"bastionPassword"`
	BastionPrivateKey string `json:"bastionPrivateKey"`

	TargetAuthType   int    `validate:"required" json:"targetAuthType"`
	TargetPassword   string `json:"targetPassword"`
	TargetPrivateKey string `json:"targetPrivateKey"`
}

type ImportEcsPasswordAuthDto struct {
	TableData string `json:"tableData" binding:"required"`
	Remark    string `json:"remark"`
}

type ImportEcsPasswordAuthFailedRow struct {
	Line   int    `json:"line"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type EcsAuthIdDto struct {
	Id uint `json:"id"`
}

type UpdateEcsAuthDto struct {
	EcsAuthIdDto
	CreateEcsPasswordAuthDto
}

type EcsAuthVo struct {
	ID         uint       `json:"id"`
	Name       string     `json:"name"`
	Type       int        `json:"type"`
	Username   string     `json:"username"`
	Password   string     `json:"password"`
	PublicKey  string     `json:"publicKey"`
	Port       int        `json:"port"`
	CreateTime util.HTime `json:"createTime"`
	UpdateTime util.HTime `json:"updateTime"`
	Remark     string     `json:"remark"`

	BastionHost       string `json:"bastionHost"`
	BastionPort       int    `json:"bastionPort"`
	BastionUsername   string `json:"bastionUsername"`
	BastionAuthType   int    `json:"bastionAuthType"`
	BastionPassword   string `json:"bastionPassword"`
	BastionPrivateKey string `json:"bastionPrivateKey"`
	TargetAuthType    int    `json:"targetAuthType"`
	TargetPassword    string `json:"targetPassword"`
	TargetPrivateKey  string `json:"targetPrivateKey"`

	HasPassword          bool `json:"hasPassword"`
	HasPublicKey         bool `json:"hasPublicKey"`
	HasBastionPassword   bool `json:"hasBastionPassword"`
	HasBastionPrivateKey bool `json:"hasBastionPrivateKey"`
	HasTargetPassword    bool `json:"hasTargetPassword"`
	HasTargetPrivateKey  bool `json:"hasTargetPrivateKey"`
}

type EcsAuthSecretVo struct {
	Password          string `json:"password"`
	PublicKey         string `json:"publicKey"`
	BastionPassword   string `json:"bastionPassword"`
	BastionPrivateKey string `json:"bastionPrivateKey"`
	TargetPassword    string `json:"targetPassword"`
	TargetPrivateKey  string `json:"targetPrivateKey"`
}
