"""Code-derived pieces of the interest model (game_plugin 26.3.2), evaluated from save bytes."""
import json, struct, datetime, sys
sys.path.insert(0, '../player-interest')
from r2 import LEVEL, score_level, reason2
TODAY = (2036, 16)
def ring(core_hex):
    b = bytes.fromhex(core_hex or '')
    if len(b) < 30: return []
    n = b[26]; out = []
    for k in range(n):
        it = b[30 + 29*k: 30 + 29*(k+1)]
        if len(it) < 29: break
        d4 = struct.unpack_from('<I', it, 17)[0]
        out.append(dict(type=it[1], id=struct.unpack_from('<H', it, 2)[0], st=struct.unpack_from('<b', it, 4)[0],
                        open=(d4 & 0x1ff) == 1 and (d4 >> 16) == 1900, u14=struct.unpack_from('<H', it, 21)[0]))
    return out
def skip3755478(i): return 900 <= i <= 930 or i in (1100, 1104, 1106) or i == 0
U14SET = {2, 18, 34, 39, 42, 60}
def ring_exempt(items):
    """0x12ae024 lines 12ae048..12ae160: an open qualifying item and no open item with u14 in the set -> 0 (exempt)."""
    q = False
    for it in items:
        if not it['open'] or it['st'] < -2 or skip3755478(it['id']): continue
        t = it['type']
        if (it['st'] != -2 and t in (19, 20)) or t in (8, 37): continue
        q = True; break
    if not q: return False
    for it in items:
        if it['open'] and (it['u14'] in U14SET or 0 <= it['u14'] - 238 < 3): return False
    return True
def ring_cap8(items):
    return any(it['open'] and not skip3755478(it['id']) and it['st'] >= 2 and it['type'] not in (19, 20, 8, 37) for it in items)
def days_since(d, y): return (TODAY[0] - y) * 365 + (TODAY[1] - d)
def just_joined(r, o, L, kind, due, clauses):
    """0x128f550 for FULL contracts; returns (P, why). Kind 3 adds the vt+0x290/0x39410f4/list checks (domestic only; in-memory parts assumed off)."""
    if r['ContractTeam'] < 0: return 0, 'free agent'
    if r['LoanTeam'] >= 0: return 0, 'on loan'
    ht = r['HdrType']; items = ring(r.get('Core'))
    days = days_since(*r['EC']) if r['EC'][1] else 99999
    if kind == 3:
        curnat = r['TeamClubNation'] if r['TeamClubNation'] >= 0 else r['ContractClubNation']
        if r['Nation'] != curnat: return 0, 'loan: not a national of his club\'s nation (vt+0x290)'
    # 0x12ae024
    if ring_exempt(items): return 0, 'exempt: open ring item (0x12ae024)'
    if set(clauses) & {55, 56}: return 0, 'exempt: clause 55/56'
    if ht == 3: return 0, 'exempt: contract type 3'
    if days >= 61 and (ht == 4 or (due is not None and due < 300)): return 0, f'exempt: ends in {due} d'
    sy, sd = r['Signed'][1], r['Signed'][0]
    s = days_since(sd, sy) if sy else 99999
    if kind == 3 and o['Age'] <= 25: T, d, mode = 250, days, 'just joined'
    elif s < days and due is not None and due >= 300: T, d, mode = 100, s, 'just signed'
    else: T, d, mode = 250, days, 'just joined'
    if T - d <= 0: return 0, 'settled'
    orig_ok = 1 <= r['Orig'] <= 6 or 15 <= r['Orig'] <= 20
    if not (r['FTManager'] >= 0 and orig_ok):
        if r['F57'] & 1 or r['F57'] & 8: return 0, 'exempt: +0x57 bit 0/3'
    P = (11 - L) * (T - d + 1) * r['Loyalty'] * (1000 if ht == 2 else 100)
    return P, f'{mode} {d} d'
def pt_value(r, o, L, kind, Rt=7218):
    """0x12ad438 for a player at another club (w25 = 0), target first team rep Rt, offered level L. Division-tier x5 term omitted."""
    if r['ContractTeam'] < 0: return None, 'free agent'
    b = o['B']; m = max(LEVEL[b-1] if 1 <= b <= 22 else 0, score_level(o))
    if ring_cap8(ring(r.get('Core'))): m = min(m, 8)
    if m == 6: w = 7 if o['GK'] > 14 else 6
    elif m == 4: w = 7
    else: w = m
    if kind in (3, 8) and o['Age'] > 20: w = max(w, 6)
    d = L - w
    CR = o['CR']; Rcur = o['ClubFirstTeamRep'] or 1500
    w23 = 5 * (Rt - CR)
    if w23 < 0 and Rt < Rcur: w23 += 10 * (Rt - Rcur)
    elif w23 >= 0: pass
    if d >= 1: v = max(0, w23 + d * CR)
    elif d == 0: v = 0
    else:
        v = d * CR - w23
        if L <= 6: v = None  # L<=6 multipliers not modelled
        else: v = min(v, 0)
    return v, f'm={m} desired={w} d={d} CR={CR}'
def r2_loan(o, Rt=7218):
    T = min(4 * Rt, 36000); w = T - min(4 * o['WR'], 36000)
    if w < -500: w *= 8
    return min(w, 0)
