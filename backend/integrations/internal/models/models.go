package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OAuthToken struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Provider     string             `bson:"provider"      json:"provider"`
	OrgID        string             `bson:"org_id"        json:"org_id"`
	AccountID    string             `bson:"account_id"    json:"account_id"`
	AccountEmail string             `bson:"account_email" json:"account_email"`
	AccountName  string             `bson:"account_name"  json:"account_name"`
	IsPrimary    bool               `bson:"is_primary"    json:"is_primary"`
	AccessToken  string             `bson:"access_token"  json:"-"`
	RefreshToken string             `bson:"refresh_token" json:"-"`
	TokenType    string             `bson:"token_type"    json:"token_type"`
	Expiry       time.Time          `bson:"expiry"        json:"expiry"`
	Scopes       []string           `bson:"scopes"        json:"scopes"`
	ConnectedAt  time.Time          `bson:"connected_at"  json:"connected_at"`
}

type FormWatch struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"    json:"id"`
	Provider       string             `bson:"provider,omitempty" json:"provider,omitempty"`
	OrgID          string             `bson:"org_id"           json:"org_id"`
	FormID         string             `bson:"form_id"          json:"form_id"`
	WorkflowID     string             `bson:"workflow_id"      json:"workflow_id"`
	Active         bool               `bson:"active"           json:"active"`
	FieldMapping   map[string]string  `bson:"field_mapping"    json:"field_mapping"`
	LastPolledAt   time.Time          `bson:"last_polled_at"   json:"last_polled_at"`
	LastResponseTS string             `bson:"last_response_ts" json:"last_response_ts"`
	CreatedAt      time.Time          `bson:"created_at"       json:"created_at"`
}

type GmailWatch struct {
	ID                    primitive.ObjectID `bson:"_id,omitempty"             json:"id"`
	OrgID                 string             `bson:"org_id"                    json:"org_id"`
	WorkflowID            string             `bson:"workflow_id"               json:"workflow_id"`
	Query                 string             `bson:"query"                     json:"query"`
	Active                bool               `bson:"active"                    json:"active"`
	LastMessageInternalTS int64              `bson:"last_message_internal_ts"  json:"last_message_internal_ts"`
	LastPolledAt          time.Time          `bson:"last_polled_at"            json:"last_polled_at"`
	CreatedAt             time.Time          `bson:"created_at"                json:"created_at"`
}
