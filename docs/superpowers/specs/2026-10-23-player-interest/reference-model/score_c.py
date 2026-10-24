import json,sys,collections,datetime
sys.path.insert(0,'.')
from code_rules import *
from v2eval import info, evaluate
raw={json.loads(l)['UID']:json.loads(l) for l in open('v2_raw.jsonl')}
tm=json.load(open('tm_recs2.json')); byid={}
for r in tm: byid.setdefault(r['id'],[]).append(r)
def due(r):
    if r['ContractTeam']<0 or r['End'][1] in (0,1900): return None
    d=datetime.date(r['End'][1],1,1)+datetime.timedelta(r['End'][0]-1); return (d-datetime.date(2036,1,16)).days
def arranged(r): return any(x['st']==19 and x['person']==r['Index'] for i in (r['IDs'] or []) for x in byid.get(i,[]))
LEFT={2002115328}  # reason-100 gate: ACTUAL_PERSON +0xd8 previous club = target (bytes), joined new club 3 days ago
def model(u):
    r=raw[u]; o=info[u]; cl=o['Clauses'] or []
    out=dict(name=o['Name'])
    if arranged(r): out.update(T=None,L=None,why='arranged transfer (status 19)'); return out
    if u in LEFT: out.update(T=None,L=None,why='reason 100: recently left Metz'); return out
    if r['ContractTeam']<0: out.update(T='?',L='?',why='free agent (path 0x1284858)'); return out
    r2=reason2(o) or 0
    PT,_=just_joined(r,o,10,4,due(r),cl); PL,_=just_joined(r,o,7,3,due(r),cl)
    v10,_=pt_value(r,o,10,4); v10b=None
    # L>=9, d>=2: d drops by 1 when L exceeds E by >1 (E unknown) -> range
    import copy
    pv7,_=pt_value(r,o,7,3)
    out.update(P_T=PT,r2=r2,pt10=v10,P_L=PL,pt7=pv7,r2L=r2_loan(o))
    out['T']= -PT + r2 + (v10 or 0)
    out['L']= -PL + (pv7 or 0) + r2_loan(o)
    out['why']=''
    return out
def decide(m,thT,thL):
    if m['T'] is None: return 'fail',m['why']
    if m['T']=='?': return '?',m['why']
    t=m['T']>=thT; l=m['L']>=thL
    return ('pass' if (t or l) else 'fail'), f"T={m['T']} (P_T {m['P_T']}, r2 {m['r2']}, pt10 {m['pt10']}) L={m['L']} (P_L {m['P_L']}, pt7 {m['pt7']})"
if __name__=='__main__':
    v1={2002078283:'pass',2002078285:'pass',2002082525:'pass',2002098107:'pass',2002099764:'pass',2002114039:'pass',2002114070:'pass',2002114299:'pass',2002114387:'pass',2002114633:'pass',2002115040:'pass',2002116366:'pass',2002123791:'pass',2002115106:'pass',2002125884:'pass',2002099116:'fail',2002099882:'fail',2002112631:'fail',2002114035:'fail',2002114155:'fail',2002115333:'fail',2002115699:'fail',2002100676:'fail'}
    v2={r['uid']:r['gt'] for r in json.load(open('v2_41_eval.json'))}
    res={}
    for nm,S,thT,thL in (('V1 Doubtful',v1,0,0),('V2 Persuadable',v2,10000,5000)):
        cm=collections.Counter(); print('==',nm)
        for u,g in S.items():
            m=model(u); p,why=decide(m,thT,thL); cm[(p,g)]+=1
            print(('OK  ' if p==g else 'MISS' if p!='?' else '??  '),g.ljust(4),p.ljust(4),m['name'][:22].ljust(22),why)
            res[u]=dict(m,pred=p,gt=g,set=nm)
        print(dict(cm))
    json.dump(res,open('score_c.json','w'),ensure_ascii=False)
