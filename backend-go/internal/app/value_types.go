package app

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type Amount struct {
	decimal.Decimal
}

func MustAmount(value string) Amount {
	amount, err := ParseAmount(value)
	if err != nil {
		panic(err)
	}
	return amount
}

func ParseAmount(value string) (Amount, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return Amount{}, errors.New("amount is required")
	}
	d, err := decimal.NewFromString(value)
	if err != nil {
		return Amount{}, err
	}
	return Amount{d.Round(2)}, nil
}

func (a Amount) MarshalJSON() ([]byte, error) {
	return []byte(a.Decimal.StringFixed(2)), nil
}

func (a *Amount) UnmarshalJSON(data []byte) error {
	raw := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if raw == "" || raw == "null" {
		return errors.New("amount is required")
	}
	parsed, err := ParseAmount(raw)
	if err != nil {
		return err
	}
	*a = parsed
	return nil
}

func (a Amount) Value() (driver.Value, error) {
	return a.Decimal.StringFixed(2), nil
}

func (a *Amount) Scan(value any) error {
	var raw string
	switch v := value.(type) {
	case nil:
		return errors.New("amount cannot be null")
	case []byte:
		raw = string(v)
	case string:
		raw = v
	default:
		raw = fmt.Sprint(v)
	}
	d, err := decimal.NewFromString(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	a.Decimal = d.Round(2)
	return nil
}

type LocalDate struct {
	time.Time
}

func Today() LocalDate {
	now := time.Now()
	return LocalDate{time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())}
}

func ParseLocalDate(value string) (LocalDate, error) {
	t, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return LocalDate{}, err
	}
	return LocalDate{t}, nil
}

func (d LocalDate) MarshalJSON() ([]byte, error) {
	if d.Time.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.Format("2006-01-02"))
}

func (d *LocalDate) UnmarshalJSON(data []byte) error {
	raw := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if raw == "" || raw == "null" {
		d.Time = time.Time{}
		return nil
	}
	parsed, err := ParseLocalDate(raw)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

func (d LocalDate) Value() (driver.Value, error) {
	if d.Time.IsZero() {
		return nil, nil
	}
	return d.Format("2006-01-02"), nil
}

func (d *LocalDate) Scan(value any) error {
	switch v := value.(type) {
	case time.Time:
		d.Time = time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.Local)
		return nil
	case []byte:
		parsed, err := ParseLocalDate(string(v))
		if err != nil {
			return err
		}
		*d = parsed
		return nil
	case string:
		parsed, err := ParseLocalDate(v)
		if err != nil {
			return err
		}
		*d = parsed
		return nil
	default:
		return fmt.Errorf("unsupported LocalDate value %T", value)
	}
}

type JSONTime struct {
	time.Time
}

func (t JSONTime) MarshalJSON() ([]byte, error) {
	if t.Time.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(t.Format("2006-01-02 15:04:05"))
}

func (t JSONTime) Value() (driver.Value, error) {
	if t.Time.IsZero() {
		return nil, nil
	}
	return t.Time, nil
}

func (t *JSONTime) Scan(value any) error {
	switch v := value.(type) {
	case time.Time:
		t.Time = v
		return nil
	case []byte:
		return t.scanString(string(v))
	case string:
		return t.scanString(v)
	default:
		return fmt.Errorf("unsupported JSONTime value %T", value)
	}
}

func (t *JSONTime) scanString(value string) error {
	layouts := []string{"2006-01-02 15:04:05.999999", "2006-01-02 15:04:05"}
	for _, layout := range layouts {
		parsed, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			t.Time = parsed
			return nil
		}
	}
	return fmt.Errorf("invalid time: %s", value)
}
