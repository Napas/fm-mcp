import json,datetime,collections,sys,unicodedata
sys.path.insert(0,'../player-interest')
from r2 import reason2, EU, LEVEL
TODAY=datetime.date(2036,1,16)
def date(d,y):
    if y in (0,1900): return None
    return datetime.date(y,1,1)+datetime.timedelta(d-1)
tm=json.load(open('tm_recs2.json')); byid={}
for r in tm: byid.setdefault(r['id'],[]).append(r)
raw={}
for l in open('v2_raw.jsonl'):
    o=json.loads(l); raw[o['UID']]=o
info={}
for l in open('../player-interest/kz16_players.jsonl'):
    o=json.loads(l)
    if o['UID'] in raw: info[o['UID']]=o
def evaluate(uid):
    r=raw[uid]; o=info[uid]
    res=dict(uid=uid,name=o['Name'],club=o['Club'],age=o['Age'],DC=o['DC'],nation=o['Nation'],know=o['Know'],loy=r['Loyalty'])
    arr=[(i,x['st'],x['t1'],x['d3']) for i in (r['IDs'] or []) for x in byid.get(i,[]) if x['person']==r['Index']]
    res['arranged']=[a for a in arr if a[1]==19]
    j=date(*r['EC']); res['joined']=j.isoformat() if j else None
    days=(2036-r['EC'][1])*365+(16-r['EC'][0]) if j else 99999
    res['days']=days
    end=date(*r['End']) if r['ContractTeam']>=0 else None
    due=(end-TODAY).days if end else None
    res['due']=due
    signed=date(*r['Signed']) if r['ContractTeam']>=0 else None
    s=(2036-signed.year)*365+(16-signed.timetuple().tm_yday) if signed else 99999
    full = r['ContractTeam']>=0 and r['HdrType'] in (0,1,2,3,4)
    onloan = r['LoanTeam']>=0
    def pen(L,kind):
        if r['ContractTeam']<0: return 0,'free agent'
        if onloan: return 0,'on loan (current contract not FULL)'
        ht=r['HdrType']
        if ht==3: return 0,'contract type 3'
        if set(o['Clauses'] or [])&{55,56}: return 0,'exempt: clause 55/56'
        if days>=61 and (ht==4 or (due is not None and due<300)): return 0,f'exempt: ends in {due} d'
        if kind==3 and o['Age']<=25: T,d,mode=250,days,'just joined'
        elif s<days:   # renewed after joining
            if due is not None and due>=300: T,d,mode=100,s,'just signed'
            else: T,d,mode=250,days,'just joined'
        else: T,d,mode=250,days,'just joined'
        if T-d<=0: return 0,'settled'
        P=(11-L)*(T-d+1)*r['Loyalty']*(1000 if ht==2 else 100)
        return P,f'{mode} {d} d'
    PT,why=pen(10,4); res['P_T10']=PT; res['why']=why
    # loan path: needs LEVEL[b] >= 5 and vt+0x290 (player's nationality == current club's nation, or kind-2/type-9 tie, or flags bit 2 + more)
    b=r['B']; lvl=LEVEL[b-1] if 1<=b<=22 else 0
    curnat=r['TeamClubNation'] if r['TeamClubNation']>=0 else r['ContractClubNation']
    rel=bytes.fromhex(r['Rel']); ties=any(rel[i+10]==2 and rel[i+11]==9 for i in range(0,len(rel),16))
    dom = r['Nation']==curnat
    flag2 = bool(r['Flags']&4)
    if lvl<5: loanapp='no (LEVEL[b]<5)'
    elif dom: loanapp='yes (domestic)'
    elif ties or flag2: loanapp='maybe (tie item / flags bit 2)'
    else: loanapp='no (foreign)'
    PL,_=pen(7,3); res['P_L']=PL if not loanapp.startswith('no') else 0; res['loan_applies']=loanapp
    try: res['r2']=reason2(o)
    except Exception: res['r2']=None
    # verdict for Transfer-or-loan Persuadable
    CUT=40000
    if res['arranged']: v='FAIL: arranged transfer'
    else:
        tf = PT>=CUT
        lf = res['P_L']>=CUT
        if tf and lf: v='FAIL: just joined (transfer and loan)'
        elif tf and loanapp.startswith('maybe'): v='FAIL? just joined; loan depends on vt+0x290'
        elif tf: v='pass via loan (transfer blocked by just-joined)'
        elif PT>=12600: v='grey: transfer P in 12.6k..40k'
        else: v='no gate'
    res['verdict']=v
    return res
if __name__=='__main__':
    pass
