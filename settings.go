package main

import "strconv"

type settingsItem struct {
	label    string
	getValue func(*config) string
	setValue func(*config, string) error
	isBool   bool
}

var settingsItems = []settingsItem{
	{
		label: "Read Ahead",
		getValue: func(c *config) string {
			return strconv.FormatInt(c.ReadAhead, 10)
		},
		setValue: func(c *config, s string) error {
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return err
			}
			c.ReadAhead = n
			return nil
		},
	},
	{
		label: "Cache",
		getValue: func(c *config) string {
			return strconv.Itoa(c.Cache)
		},
		setValue: func(c *config, s string) error {
			n, err := strconv.Atoi(s)
			if err != nil {
				return err
			}
			c.Cache = n
			return nil
		},
	},
	{
		label: "No Cache",
		getValue: func(c *config) string {
			return strconv.FormatBool(c.NoCache)
		},
		setValue: func(c *config, s string) error {
			noCache, err := strconv.ParseBool(s)
			if err != nil {
				return err
			}
			c.NoCache = noCache
			return nil
		},
		isBool: true,
	},
	{
		label: "Buffer Percent",
		getValue: func(c *config) string {
			return strconv.Itoa(c.BufferPercent)
		},
		setValue: func(c *config, s string) error {
			n, err := strconv.Atoi(s)
			if err != nil {
				return err
			}
			c.BufferPercent = n
			return nil
		},
	},
}
