-- Interviews are priced on time, so rooms need to know how long they run and
-- when they started, and users need to know which tier they are on.

-- Free until somebody is moved off it by hand. There is no billing yet, so
-- this column is the whole subscription system.
ALTER TABLE users
    ADD COLUMN plan TEXT NOT NULL DEFAULT 'free';

ALTER TABLE rooms
    -- When the interviewer means to hold it. Advisory: the timer runs from
    -- when the room is actually opened, not from this.
    ADD COLUMN scheduled_at TIMESTAMPTZ,
    -- How long the interview may run once it starts. Clamped to the owner's
    -- plan at creation, so a later downgrade cannot shorten a booked session.
    ADD COLUMN duration_minutes INT NOT NULL DEFAULT 60
        CHECK (duration_minutes > 0 AND duration_minutes <= 480),
    -- Set the first time anyone joins. The deadline is started_at plus the
    -- duration, so a room created on Monday and used on Friday gets its full
    -- time — the clock measures the interview, not the booking.
    ADD COLUMN started_at TIMESTAMPTZ;

-- Counting a user's interviews is on the room-creation path, so it happens
-- before every new room.
CREATE INDEX rooms_owner_created_idx ON rooms (owner_id, created_at);
