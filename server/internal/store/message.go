package store

import (
	"context"
	"time"
	"unicode/utf8"
)

// 留言状态
const (
	MsgPending  = "pending"
	MsgApproved = "approved"
	MsgRejected = "rejected"
)

// Message 对应 messages 表的一行（完整形态，含 IP/UA，仅后台可见）。
type Message struct {
	ID             int64      `json:"id"`
	Nickname       string     `json:"nickname"`
	Content        string     `json:"content"`
	Status         string     `json:"status"`
	IP             string     `json:"ip"`
	UserAgent      string     `json:"user_agent"`
	CreatedAt      time.Time  `json:"created_at"`
	ReviewedAt     *time.Time `json:"reviewed_at"`
	ReviewedAction string     `json:"reviewed_action"`
}

// PublicMessage 是公开接口返回的留言形态，不含 IP / UA。
type PublicMessage struct {
	ID        int64     `json:"id"`
	Nickname  string    `json:"nickname"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// ToPublic 从完整留言转公开形态（丢弃 ip/user_agent/reviewed_*）。
func (m *Message) ToPublic() PublicMessage {
	return PublicMessage{
		ID:        m.ID,
		Nickname:  m.Nickname,
		Content:   m.Content,
		CreatedAt: m.CreatedAt,
	}
}

// InsertMessage 新增留言。强制 status=pending；昵称/正文校验。
// IP/UA 只信 handler 传入，不读 body。
func (s *Store) InsertMessage(ctx context.Context, nickname, content, ip, userAgent string) (*Message, error) {
	if nickname == "" || utf8.RuneCountInString(nickname) > 24 {
		return nil, ErrValidation
	}
	if content == "" || utf8.RuneCountInString(content) > 500 {
		return nil, ErrValidation
	}
	var m Message
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO messages (nickname, content, status, ip, user_agent)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at`,
		nickname, content, MsgPending, ip, userAgent).
		Scan(&m.ID, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	m.Nickname = nickname
	m.Content = content
	m.Status = MsgPending
	m.IP = ip
	m.UserAgent = userAgent
	return &m, nil
}

// ListApprovedMessages 已过审留言，按时间倒序。
func (s *Store) ListApprovedMessages(ctx context.Context) ([]PublicMessage, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, nickname, content, created_at
		 FROM messages WHERE status = $1 ORDER BY created_at DESC`,
		MsgApproved)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []PublicMessage{}
	for rows.Next() {
		var m PublicMessage
		if err := rows.Scan(&m.ID, &m.Nickname, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// ListMessagesAdmin 全部留言（含 IP/UA），按时间倒序。
func (s *Store) ListMessagesAdmin(ctx context.Context) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, nickname, content, status, ip::text, COALESCE(user_agent, ''), created_at, reviewed_at, COALESCE(reviewed_action, '')
		 FROM messages ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Nickname, &m.Content, &m.Status, &m.IP, &m.UserAgent,
			&m.CreatedAt, &m.ReviewedAt, &m.ReviewedAction); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// ReviewMessage 审核留言：action 只能是 approved / rejected。
func (s *Store) ReviewMessage(ctx context.Context, id int64, action string) error {
	if action != MsgApproved && action != MsgRejected {
		return ErrValidation
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE messages
		 SET status = $2, reviewed_at = now(), reviewed_action = $2
		 WHERE id = $1`,
		id, action)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteMessage 删除留言。0 行 → ErrNotFound。
func (s *Store) DeleteMessage(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM messages WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
