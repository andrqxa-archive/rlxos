/*
 * Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 *
 */

package settingsapi

import (
	"fmt"
	"strings"
	"time"

	"avyos.dev/pkg/fs"
	"avyos.dev/pkg/sutra"
)

const (
	RequestGet  = RequestRawGet
	RequestSet  = RequestRawSet
	RequestList = RequestRawList

	EventChanged = EventSettingsChanged
)

func Connect() (*Client, error) {
	client, err := NewClient(fs.Resolve("user-service:" + ServiceName))
	if err != nil {
		return nil, err
	}
	client.SetTimeout(4 * time.Second)
	return client, nil
}

func (c *Client) Raw() *sutra.Client {
	return c.client
}

func (c *Client) Get(key string) (string, error) {
	key, err := normalizeKey(key)
	if err != nil {
		return "", err
	}
	resp, err := c.RawGet(KeyRequest{Key: key})
	if err != nil {
		return "", err
	}
	return resp.Value, nil
}

func (c *Client) Set(key, value string) error {
	key, err := normalizeKey(key)
	if err != nil {
		return err
	}
	_, err = c.RawSet(SetRequest{Key: key, Value: value})
	return err
}

func (c *Client) List(prefix string) ([]Entry, error) {
	resp, err := c.RawList(ListRequest{Prefix: strings.TrimSpace(prefix)})
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) OnChanged(fn func(ChangedEvent)) {
	c.OnSettingsChanged(func(_ uint32, ev ChangedEvent) {
		fn(ev)
	})
}

func (c *Client) OnDisconnect(fn func()) {
	c.client.OnDisconnect(fn)
}

func EncodeKey(key string) ([]byte, error) {
	key, err := normalizeKey(key)
	if err != nil {
		return nil, err
	}
	return KeyRequest{Key: key}.MarshalBinary(), nil
}

func DecodeKey(data []byte) (string, error) {
	var req KeyRequest
	if err := req.UnmarshalBinary(data); err != nil {
		return "", err
	}
	return normalizeKey(req.Key)
}

func EncodeGetResponse(value string) []byte {
	return GetResponse{Value: value}.MarshalBinary()
}

func DecodeGetResponse(data []byte) (string, error) {
	var resp GetResponse
	if err := resp.UnmarshalBinary(data); err != nil {
		return "", err
	}
	return resp.Value, nil
}

func EncodeSetRequest(key, value string) ([]byte, error) {
	key, err := normalizeKey(key)
	if err != nil {
		return nil, err
	}
	return SetRequest{Key: key, Value: value}.MarshalBinary(), nil
}

func DecodeSetRequest(data []byte) (string, string, error) {
	var req SetRequest
	if err := req.UnmarshalBinary(data); err != nil {
		return "", "", err
	}
	key, err := normalizeKey(req.Key)
	if err != nil {
		return "", "", err
	}
	return key, req.Value, nil
}

func EncodeChangedEvent(key, value string) []byte {
	return ChangedEvent{Key: strings.TrimSpace(key), Value: value}.MarshalBinary()
}

func DecodeChangedEvent(data []byte) (ChangedEvent, error) {
	var ev ChangedEvent
	if err := ev.UnmarshalBinary(data); err != nil {
		return ChangedEvent{}, err
	}
	key, err := normalizeKey(ev.Key)
	if err != nil {
		return ChangedEvent{}, err
	}
	ev.Key = key
	return ev, nil
}

func EncodeEntryList(entries []Entry) []byte {
	return EntryList{Items: entries}.MarshalBinary()
}

func DecodeEntryList(data []byte) ([]Entry, error) {
	var list EntryList
	if err := list.UnmarshalBinary(data); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func normalizeKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("key is required")
	}
	return key, nil
}
