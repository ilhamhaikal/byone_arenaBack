package validator

import (
	"github.com/go-playground/validator/v10"
)

// Validator wrapper untuk validator/v10
type Validator struct {
	validate *validator.Validate
}

// New membuat instance validator baru
func New() *Validator {
	return &Validator{validate: validator.New()}
}

// ValidateStruct memvalidasi struct berdasarkan tag validate
func (v *Validator) ValidateStruct(s interface{}) error {
	return v.validate.Struct(s)
}

// FormatError mengkonversi error validasi menjadi pesan yang mudah dibaca
func FormatError(err error) string {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrors {
			switch e.Tag() {
			case "required":
				return "Field '" + e.Field() + "' wajib diisi"
			case "min":
				return "Field '" + e.Field() + "' minimal " + e.Param() + " karakter"
			case "max":
				return "Field '" + e.Field() + "' maksimal " + e.Param() + " karakter"
			case "email":
				return "Format email tidak valid"
			case "oneof":
				return "Field '" + e.Field() + "' harus salah satu dari: " + e.Param()
			case "gt":
				return "Field '" + e.Field() + "' harus lebih besar dari " + e.Param()
			}
		}
		return err.Error()
	}
	return err.Error()
}
