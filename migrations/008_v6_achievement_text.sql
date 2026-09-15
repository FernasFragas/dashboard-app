-- The v6 plan shifted every Phase 1 week code by +4 and renamed two chaos tasks, so the
-- achievement copy naming week numbers went stale. The codes are load-bearing (internal/api
-- /game.go keys off them) and are deliberately left alone: only the descriptions move.

UPDATE achievements SET description = 'Complete the W7 regression-gate-in-ci task.'
    WHERE code = 'gatekeeper';
UPDATE achievements SET description = 'Complete the five W9-W10 chaos tasks.'
    WHERE code = 'chaos_suite';
UPDATE achievements SET description = 'Publish the W14 honest number.'
    WHERE code = 'honest_number';
UPDATE achievements SET description = 'Open the W2 pull request.'
    WHERE code = 'in_the_arena';
UPDATE achievements SET description = 'Submit the W16 checkpoint review.'
    WHERE code = 'boss_w12';

PRAGMA foreign_key_check;
