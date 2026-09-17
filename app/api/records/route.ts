type Row={id:number;name:string;display:string;category:string;owner:string;status:boolean;updated:string};
const initial:Row[]=[{id:1,name:"admin",display:"管理员",category:"超级管理员",owner:"研发总监",status:true,updated:"2026-09-17 17:30"},{id:2,name:"test",display:"游客",category:"test",owner:"测试工程师",status:true,updated:"2026-09-16 14:22"}];
const state=globalThis as typeof globalThis&{autoOpsRows?:Row[]};
function rows(){return state.autoOpsRows??=(structuredClone(initial))}
export async function GET(){return Response.json({rows:rows()})}
export async function POST(request:Request){const body=await request.json() as Partial<Row>;const row:Row={id:Math.max(0,...rows().map(r=>r.id))+1,name:String(body.name||""),display:String(body.display||body.name||""),category:String(body.category||"默认"),owner:String(body.owner||"admin"),status:body.status!==false,updated:new Date().toLocaleString("zh-CN",{hour12:false})};rows().push(row);return Response.json({row},{status:201})}
export async function PUT(request:Request){const body=await request.json() as Row;const i=rows().findIndex(r=>r.id===body.id);if(i<0)return Response.json({message:"记录不存在"},{status:404});rows()[i]={...rows()[i],...body,updated:new Date().toLocaleString("zh-CN",{hour12:false})};return Response.json({row:rows()[i]})}
export async function DELETE(request:Request){const id=Number(new URL(request.url).searchParams.get("id"));const i=rows().findIndex(r=>r.id===id);if(i<0)return Response.json({message:"记录不存在"},{status:404});rows().splice(i,1);return Response.json({ok:true})}
