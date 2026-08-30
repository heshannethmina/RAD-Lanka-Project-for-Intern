-- Promotion codes: a redeemable grant that lifts somebody's limits without a
-- subscription, for pilots, universities and anyone being given the product.
--
-- The code is stored in plain text, unlike session and invite tokens, which
-- are hashed. That is deliberate and the reasoning is worth writing down: a
-- token identifies a *person* and must be useless to whoever steals the
-- database, whereas a promo code is a coupon — it is printed on a slide, typed
-- by hand, and often shared with a whole cohort on purpose. Hashing it would
-- mean an operator could never read back a code they issued, and could not
-- write one with a plain INSERT. The blast radius of a leak is free interview
-- minutes, which max_redemptions and expires_at already bound.
CREATE TABLE promo_codes (
    -- Normalised to upper case on the way in, so "syncr-pilot" and
    -- "SYNCR-PILOT" are the same coupon rather than two.
    code TEXT PRIMARY KEY
        CHECK (code = upper(code) AND char_length(code) BETWEEN 4 AND 64),
    -- Which tier the code grants. Named rather than fixed at "unlimited", so
    -- "three months of Pro" is a row and not a code change.
    plan TEXT NOT NULL DEFAULT 'unlimited',
    -- How many people may redeem it. 0 means no ceiling.
    max_redemptions INT NOT NULL DEFAULT 0 CHECK (max_redemptions >= 0),
    redemptions     INT NOT NULL DEFAULT 0 CHECK (redemptions >= 0),
    -- When the code stops being redeemable. NULL means never. This bounds the
    -- coupon, not the grant it hands out.
    expires_at TIMESTAMPTZ,
    -- How long the grant lasts once redeemed. NULL means it does not lapse.
    grant_days INT CHECK (grant_days IS NULL OR grant_days > 0),
    -- Free text for whoever has to work out later why this code exists.
    note       TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One row per person per code, and the primary key is the whole point: a code
-- with grant_days set could otherwise be redeemed again every morning to push
-- the expiry out forever, which would make a 30-day trial permanent.
CREATE TABLE promo_redemptions (
    code        TEXT   NOT NULL REFERENCES promo_codes (code) ON DELETE CASCADE,
    user_id     BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    redeemed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (code, user_id)
);

-- The grant lives on the user, beside the plan rather than replacing it: a
-- promo is an override with an end date, so when it lapses somebody falls back
-- to whatever they were actually paying for instead of being dropped to Free.
--
-- promo_code carries no foreign key on purpose. It is a record of what was
-- redeemed, kept for support; deleting a leaked coupon should not quietly
-- rewrite the grants already handed out. Revoking those is a separate,
-- deliberate UPDATE.
ALTER TABLE users
    ADD COLUMN promo_code       TEXT,
    ADD COLUMN promo_plan       TEXT,
    ADD COLUMN promo_expires_at TIMESTAMPTZ;
