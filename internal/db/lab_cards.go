package db

import (
	"database/sql"
	"errors"
	"time"
)

var ErrLabCardConfirmed = errors.New("lab card already confirmed")

type LabCardDraft struct {
	Number            int        `json:"number"`
	Filename          string     `json:"filename"`
	StoragePath       string     `json:"-"`
	SHA256            string     `json:"sha256"`
	FileSize          int64      `json:"file_size"`
	SuggestedDeadline time.Time  `json:"suggested_deadline"`
	Status            string     `json:"status"`
	DeliveryStatus    string     `json:"delivery_status"`
	UploadedAt        time.Time  `json:"uploaded_at"`
	ConfirmedAt       *time.Time `json:"confirmed_at,omitempty"`
}

func (d *DB) SaveLabCardDraft(card LabCardDraft) error {
	result, err := d.conn.Exec(`INSERT INTO lab_card_drafts
		(number, filename, storage_path, sha256, file_size, suggested_deadline, status, delivery_status, uploaded_at)
		VALUES (?, ?, ?, ?, ?, ?, 'draft', 'not_started', ?)
		ON CONFLICT(number) DO UPDATE SET
		filename=excluded.filename, storage_path=excluded.storage_path,
		sha256=excluded.sha256, file_size=excluded.file_size,
		suggested_deadline=excluded.suggested_deadline, uploaded_at=excluded.uploaded_at
		WHERE lab_card_drafts.status='draft'`,
		card.Number, card.Filename, card.StoragePath, card.SHA256, card.FileSize,
		card.SuggestedDeadline.UTC().Format(time.RFC3339Nano), card.UploadedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return ErrLabCardConfirmed
	}
	return nil
}

func (d *DB) GetLabCardDraft(number int) (*LabCardDraft, error) {
	var card LabCardDraft
	var deadline, uploaded string
	var confirmed sql.NullString
	err := d.conn.QueryRow(`SELECT number, filename, storage_path, sha256, file_size,
		suggested_deadline, status, delivery_status, uploaded_at, confirmed_at
		FROM lab_card_drafts WHERE number=?`, number).
		Scan(&card.Number, &card.Filename, &card.StoragePath, &card.SHA256, &card.FileSize,
			&deadline, &card.Status, &card.DeliveryStatus, &uploaded, &confirmed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	card.SuggestedDeadline, err = time.Parse(time.RFC3339Nano, deadline)
	if err != nil {
		return nil, err
	}
	card.UploadedAt, err = time.Parse(time.RFC3339Nano, uploaded)
	if err != nil {
		return nil, err
	}
	if confirmed.Valid {
		at, parseErr := time.Parse(time.RFC3339Nano, confirmed.String)
		if parseErr != nil {
			return nil, parseErr
		}
		card.ConfirmedAt = &at
	}
	return &card, nil
}

func (d *DB) ConfirmLabCard(number int, now time.Time) error {
	result, err := d.conn.Exec(`UPDATE lab_card_drafts SET status='confirmed', delivery_status='manual_pending', confirmed_at=?
		WHERE number=? AND status='draft'`, now.UTC().Format(time.RFC3339Nano), number)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return ErrNotFound
	}
	return nil
}
