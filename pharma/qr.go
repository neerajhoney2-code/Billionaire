package pharma

import (
	"fmt"
	"image/color"
	"image/png"
	"io"

	qrcode "github.com/skip2/go-qrcode"
)

// GenerateQRPNG writes a QR code PNG for the medicine verify URL to w.
// The QR encodes: https://<host>/verify/<qrHash>
func GenerateQRPNG(qrHash, host string, w io.Writer) error {
	url := fmt.Sprintf("http://%s/verify/%s", host, qrHash)

	qr, err := qrcode.New(url, qrcode.High)
	if err != nil {
		return fmt.Errorf("create qr: %w", err)
	}
	qr.BackgroundColor = color.White
	qr.ForegroundColor = color.Black

	img := qr.Image(256)
	return png.Encode(w, img)
}

// VerifyURL returns the URL that the QR code encodes.
func VerifyURL(qrHash, host string) string {
	return fmt.Sprintf("http://%s/verify/%s", host, qrHash)
}
