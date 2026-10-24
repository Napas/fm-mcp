import json, sys, unicodedata
EU = {'Austria','Belgium','Bulgaria','Croatia','Cyprus','Czech Republic','Czechia','Denmark','Estonia','Finland','France','Germany','Greece','Hungary','Republic of Ireland','Ireland','Italy','Latvia','Lithuania','Luxembourg','Malta','Netherlands','Holland','Poland','Portugal','Romania','Slovakia','Slovenia','Spain','Sweden'}
LEVEL = [10, 9, 8, 7, 7, 7, 6, 6, 5, 4, 4, 4, 3, 2, 8, 7, 7, 7, 7, 7, 5, 1]  # 0x5203960, status 1..22
METZ_REP = 7218
def norm(s): return ''.join(c for c in unicodedata.normalize('NFD', s.lower()) if unicodedata.category(c) != 'Mn')
def score_level(o, youth=False):
    s, b = o['Score'], o['B']; keeper = o['GK'] >= 18; cupk = 16 <= b <= 19
    if s >= 85 and b == 1: return 10
    if s >= 85: return 9
    if s >= 60: return 8
    if (cupk and o['Recent'] >= 0) or (s >= 30 and not keeper): return 7
    if s >= 10: return 4 if youth else 6
    if youth: return 3
    if s >= -10: return 5
    return 1
def reason2(o, tgt_rep=METZ_REP, kind=4):
    """reason-2 value (0x128c898) for a club player with a FULL contract at another club; None when not modelled."""
    if o['ClubFirstTeam'] < 0 or o['B'] == 0: return None
    T = min(4 * tgt_rep, 36000)
    C = min(4 * o['ClubFirstTeamRep'], 36000)
    if o['Age'] >= 30: pass  # 0x2f9e25c may give x1.1 (not modelled)
    elif 63 in (o['Clauses'] or []) or 54 in (o['Clauses'] or []): T = int(T * 1.15)
    m = max(LEVEL[o['B'] - 1], score_level(o))
    if m <= 6 and m != 4: C = int(C * 0.8)
    d = 0
    if kind not in (3, 8):
        d = T - C
        d = d * 6 if d <= -4001 else d * 5 if d <= -3001 else d * 4 if d <= -2001 else d * 2 if d < -1000 else d
    w = T - min(4 * o['WR'], 36000)
    if w < -500: w *= 8
    w = min(w, 0)
    return d + w
