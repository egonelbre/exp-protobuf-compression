package pbmodel

import (
	"bytes"
	"testing"

	"github.com/egonelbre/vibing/pbmodel/testdata"
	"google.golang.org/protobuf/proto"
)

func TestAdaptiveRoundtrip(t *testing.T) {
	tests := []struct {
		name string
		msg  proto.Message
	}{
		{
			name: "simple message",
			msg: &testdata.SimpleMessage{
				Id:     12345,
				Name:   "Alice Smith",
				Active: true,
			},
		},
		{
			name: "user profile",
			msg: &testdata.UserProfile{
				UserId:   67890,
				Username: "bob",
				Email:    "bob@test.org",
				FullName: "Bob Johnson",
				Bio:      "Software engineer with a passion for distributed systems and open source.",
			},
		},
		{
			name: "user with tags",
			msg: &testdata.UserProfile{
				UserId:   11111,
				Username: "charlie",
				Email:    "charlie@dev.io",
				FullName: "Charlie Brown",
				Tags:     []string{"golang", "rust", "python", "kubernetes"},
			},
		},
		{
			name: "user with address",
			msg: &testdata.UserProfile{
				UserId:   22222,
				Username: "diana",
				Email:    "diana@hero.com",
				FullName: "Diana Prince",
				Address: &testdata.UserProfile_Address{
					Street:  "123 Main Street",
					City:    "Metropolis",
					State:   "NY",
					Zip:     "10001",
					Country: "USA",
				},
			},
		},
		{
			name: "nested message",
			msg: &testdata.NestedMessage{
				Inner: &testdata.NestedMessage_Inner{
					Value: "inner value",
					Count: 42,
				},
				InnerList: []*testdata.NestedMessage_Inner{
					{Value: "first", Count: 1},
					{Value: "second", Count: 2},
				},
				OuterField: "outer value",
			},
		},
		{
			name: "complete user profile",
			msg: &testdata.UserProfile{
				UserId:   99999,
				Username: "eve",
				Email:    "eve@complete.test",
				FullName: "Eve Wilson",
				Bio:      "Senior architect specializing in cloud infrastructure and microservices. Passionate about mentoring junior developers.",
				Tags:     []string{"cloud", "aws", "terraform", "docker"},
				Address: &testdata.UserProfile_Address{
					Street:  "456 Oak Avenue",
					City:    "San Francisco",
					State:   "CA",
					Zip:     "94102",
					Country: "United States",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Compress
			var buf bytes.Buffer
			err := AdaptiveCompress(tt.msg, &buf)
			if err != nil {
				t.Fatalf("AdaptiveCompress failed: %v", err)
			}

			// Decompress
			result := tt.msg.ProtoReflect().New().Interface()
			err = AdaptiveDecompress(&buf, result)
			if err != nil {
				t.Fatalf("AdaptiveDecompress failed: %v", err)
			}

			// Compare
			if !proto.Equal(tt.msg, result) {
				t.Errorf("Messages differ:\noriginal: %v\nresult:   %v", tt.msg, result)
			}
		})
	}
}

func TestAdaptiveWithNestedMessages(t *testing.T) {
	msg := &testdata.UserProfile{
		UserId:   1,
		Username: "testuser",
		Email:    "test@example.com",
		FullName: "Test User",
		Address: &testdata.UserProfile_Address{
			Street:  "Main Street",
			City:    "TestCity",
			State:   "TS",
			Zip:     "12345",
			Country: "TestCountry",
		},
	}

	var buf bytes.Buffer
	err := AdaptiveCompress(msg, &buf)
	if err != nil {
		t.Fatalf("AdaptiveCompress failed: %v", err)
	}

	result := &testdata.UserProfile{}
	err = AdaptiveDecompress(&buf, result)
	if err != nil {
		t.Fatalf("AdaptiveDecompress failed: %v", err)
	}

	if !proto.Equal(msg, result) {
		t.Errorf("Messages differ:\noriginal: %v\nresult:   %v", msg, result)
	}
}

func TestAdaptiveWithRepeatedFields(t *testing.T) {
	msg := &testdata.UserProfile{
		UserId:   42,
		Username: "repeatedtester",
		Email:    "repeated@test.com",
		FullName: "Repeated Tester",
		Tags:     []string{"tag1", "tag2", "tag3", "tag4", "tag5"},
	}

	var buf bytes.Buffer
	err := AdaptiveCompress(msg, &buf)
	if err != nil {
		t.Fatalf("AdaptiveCompress failed: %v", err)
	}

	result := &testdata.UserProfile{}
	err = AdaptiveDecompress(&buf, result)
	if err != nil {
		t.Fatalf("AdaptiveDecompress failed: %v", err)
	}

	if !proto.Equal(msg, result) {
		t.Errorf("Messages differ:\noriginal: %v\nresult:   %v", msg, result)
	}
}

func TestAdaptiveEmptyMessage(t *testing.T) {
	msg := &testdata.EmptyMessage{}

	var buf bytes.Buffer
	err := AdaptiveCompress(msg, &buf)
	if err != nil {
		t.Fatalf("AdaptiveCompress failed: %v", err)
	}

	result := &testdata.EmptyMessage{}
	err = AdaptiveDecompress(&buf, result)
	if err != nil {
		t.Fatalf("AdaptiveDecompress failed: %v", err)
	}

	if !proto.Equal(msg, result) {
		t.Errorf("Messages differ:\noriginal: %v\nresult:   %v", msg, result)
	}
}
