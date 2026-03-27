import React from 'react';

function ExecOverview() {
    return (
        <div className="p-10 space-y-10 pb-20 h-full overflow-y-auto custom-scrollbar">
            {/* 1. Stat Cards */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
                {[
                    { l: 'การใช้งานรวม (Total Usage)', v: '32,840', t: '+12.4% จากเทอมที่แล้ว', c: 'bg-[#7c3aed]', i: 'data_usage' },
                    { l: 'อัตราใช้งานสูงสุด (Peak Util.)', v: '94.2%', t: 'จากความจุทั้งหมด 50 เครื่อง', c: 'bg-amber-400', i: 'speed' },
                    { l: 'ความพึงพอใจ (Avg. Satisfaction)', v: '4.92/5', t: 'บริการระดับยอดเยี่ยม', c: 'bg-blue-400', i: 'verified_user' },
                    { l: 'ผู้ใช้งานไม่ซ้ำ (Unique Users)', v: '4,152', t: 'Active ในเทอมนี้', c: 'bg-purple-400', i: 'groups' },
                ].map((card, i) => (
                    <div key={i} className="glass-card p-6 rounded-3xl shadow-sm relative overflow-hidden">
                        <div className={`absolute top-0 right-0 w-20 h-20 ${card.c} opacity-[0.04] rounded-full -mr-6 -mt-6`}></div>
                        <div className="flex items-center justify-between mb-4">
                            <span className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">{card.l}</span>
                            <span className="material-symbols-outlined text-lg text-slate-300">{card.i}</span>
                        </div>
                        <h3 className="text-3xl font-black text-slate-800 tracking-tighter">{card.v}</h3>
                        <p className="text-[11px] font-bold text-emerald-500 mt-2 flex items-center gap-1">
                            <span className="w-1.5 h-1.5 rounded-full bg-emerald-500"></span> {card.t}
                        </p>
                    </div>
                ))}
            </div>

            {/* 2. Term-over-Term Comparison */}
            <div className="glass-card rounded-[2.5rem] p-10 shadow-sm border border-slate-100">
                <div className="flex items-center justify-between mb-12">
                    <div>
                        <h4 className="font-bold text-slate-800 text-lg">เปรียบเทียบตารางความเติบโต (Term-over-Term)</h4>
                        <p className="text-xs text-slate-400 mt-1">เปรียบเทียบข้อมูลเชิงลึกระหว่างปี 2024 กับ 2025</p>
                    </div>
                    <div className="flex items-center gap-6 bg-slate-50 px-5 py-2.5 rounded-2xl border border-slate-100">
                        <div className="flex items-center gap-2.5">
                            <div className="w-2.5 h-2.5 rounded-full bg-[#7c3aed]"></div>
                            <span className="text-[10px] font-bold text-slate-600 uppercase">เทอม 1 / 2568</span>
                        </div>
                        <div className="flex items-center gap-2.5">
                            <div className="w-2.5 h-2.5 rounded-full bg-purple-100 border border-purple-200"></div>
                            <span className="text-[10px] font-bold text-slate-600 uppercase">เทอม 1 / 2567</span>
                        </div>
                    </div>
                </div>
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-20">
                    <div className="text-left">
                        <h5 className="text-[11px] font-bold text-slate-400 mb-8 uppercase tracking-widest">การเติบโต: ผู้เยี่ยมชมใหม่ (Unique Users)</h5>
                        <div style={{ height: '256px' }} className="flex items-end justify-between gap-12 px-6 border-b border-slate-100">
                            {[{ a: 75, b: 60 }, { a: 88, b: 68 }, { a: 100, b: 84 }].map((val, i) => (
                                <div key={i} className="flex-1 flex justify-center gap-3 items-end h-full">
                                    <div className="w-full max-w-[30px] rounded-t-lg shadow-sm" style={{ height: `${val.a}%`, background: 'linear-gradient(to top, #ede9fe, #7c3aed)' }}></div>
                                    <div className="w-full max-w-[30px] bg-purple-50 border-t border-purple-100 rounded-t-lg" style={{ height: `${val.b}%` }}></div>
                                </div>
                            ))}
                        </div>
                        <div className="flex justify-between mt-5 px-6 text-[10px] font-bold text-slate-400 uppercase">
                            <span>ส.ค.-ก.ย.</span><span>ก.ย.-ต.ค.</span><span>ต.ค.-พ.ย.</span>
                        </div>
                    </div>
                    <div className="text-left">
                        <h5 className="text-[11px] font-bold text-slate-400 mb-8 uppercase tracking-widest">การเติบโต: คะแนนความพึงพอใจ (Satisfaction)</h5>
                        <div style={{ height: '256px' }} className="flex items-end justify-between gap-12 px-6 border-b border-slate-100">
                            {[{ a: 95, b: 85 }, { a: 98, b: 92 }, { a: 96, b: 88 }].map((val, i) => (
                                <div key={i} className="flex-1 flex justify-center gap-3 items-end h-full">
                                    <div className="w-full max-w-[30px] rounded-t-lg shadow-sm" style={{ height: `${val.a}%`, background: 'linear-gradient(to top, #ede9fe, #7c3aed)' }}></div>
                                    <div className="w-full max-w-[30px] bg-purple-50 border-t border-purple-100 rounded-t-lg" style={{ height: `${val.b}%` }}></div>
                                </div>
                            ))}
                        </div>
                        <div className="flex justify-between mt-5 px-6 text-[10px] font-bold text-slate-400 uppercase">
                            <span>เดือน 1</span><span>เดือน 2</span><span>เดือน 3</span>
                        </div>
                    </div>
                </div>
            </div>

            {/* Grid Row */}
            <div className="grid grid-cols-12 gap-10">
                {/* 3. Monthly Usage Volume (Large) */}
                <div className="col-span-12 lg:col-span-8 glass-card rounded-[2.5rem] p-10 shadow-sm">
                    <div className="flex items-center justify-between mb-10 text-left">
                        <h4 className="font-bold text-slate-800">ปริมาณการใช้งานรายเดือน (Monthly Usage)</h4>
                        <span className="text-[10px] font-black text-[#7c3aed] uppercase border-b-2 border-purple-100 pb-1 tracking-widest">สถิติเทอมปัจจุบัน</span>
                    </div>
                    <div style={{ height: '256px' }} className="flex items-end justify-between gap-6 px-4">
                        {[
                            { h: 45, label: 'ส.ค.' }, { h: 68, label: 'ก.ย.' }, { h: 92, label: 'ต.ค.' },
                            { h: 80, label: 'พ.ย.' }, { h: 100, label: 'ธ.ค.' }, { h: 75, label: 'ม.ค.' }
                        ].map((bar, i) => (
                            <div key={i} className="flex-1 flex flex-col items-center gap-4 group h-full">
                                <div className="w-full bg-slate-50 rounded-t-[1.5rem] hover:bg-purple-50 transition-all cursor-pointer relative overflow-hidden flex-1">
                                    <div className="absolute bottom-0 w-full rounded-t-xl transition-all duration-700" style={{ height: `${bar.h}%`, background: 'linear-gradient(to top, #ede9fe, #a855f7)' }}></div>
                                    <div className="absolute top-1/2 left-1/2 -translate-x-1/2 w-2 h-2 bg-[#7c3aed] rounded-full opacity-0 group-hover:opacity-100 transition-opacity"></div>
                                </div>
                                <span className="text-[10px] font-bold text-slate-400 uppercase tracking-widest shrink-0">{bar.label}</span>
                            </div>
                        ))}
                    </div>
                </div>

                {/* 4. User Demographics (Donut) */}
                <div className="col-span-12 lg:col-span-4 glass-card rounded-[2.5rem] p-10 shadow-sm text-center">
                    <h4 className="font-bold text-slate-800 text-sm mb-10 text-left uppercase tracking-[0.15em]">สัดส่วนประเภทผู้ใช้งาน (Demographics)</h4>
                    <div className="flex flex-col items-center justify-center h-48 relative">
                        <svg className="w-44 h-44 -rotate-90" viewBox="0 0 36 36">
                            <circle cx="18" cy="18" r="16" fill="none" stroke="#f1f5f9" strokeWidth="3.5"></circle>
                            <circle cx="18" cy="18" r="16" fill="none" stroke="#7c3aed" strokeWidth="3.5" strokeDasharray="68 100" strokeLinecap="round"></circle>
                            <circle cx="18" cy="18" r="16" fill="none" stroke="#a78bfa" strokeWidth="3.5" strokeDasharray="22 100" strokeDashoffset="-68" strokeLinecap="round"></circle>
                            <circle cx="18" cy="18" r="16" fill="none" stroke="#cbd5e1" strokeWidth="3.5" strokeDasharray="10 100" strokeDashoffset="-90" strokeLinecap="round"></circle>
                        </svg>
                        <div className="absolute flex flex-col items-center">
                            <span className="text-3xl font-black text-slate-800 tracking-tighter">100%</span>
                            <span className="text-[9px] font-bold text-slate-400 uppercase tracking-widest">ผู้ใช้รวม (Total)</span>
                        </div>
                    </div>
                    <div className="mt-10 space-y-4 text-left px-2">
                        {[
                            { l: 'นักศึกษา (Students)', p: '68%', v: '2,823', c: 'bg-[#7c3aed]' },
                            { l: 'บุคลากร (Staff)', p: '22%', v: '913', c: 'bg-[#a78bfa]' },
                            { l: 'ภายนอก (Visitors)', p: '10%', v: '416', c: 'bg-slate-200' }
                        ].map((item, idx) => (
                            <div key={idx} className="flex items-center justify-between">
                                <div className="flex items-center gap-3">
                                    <div className={`w-2 h-2 rounded-full ${item.c}`}></div>
                                    <span className="text-[11px] font-bold text-slate-600 uppercase tracking-wide">{item.l} ({item.p})</span>
                                </div>
                                <span className="text-xs font-black text-slate-800">{item.v}</span>
                            </div>
                        ))}
                    </div>
                </div>

                {/* 5. Usage by Department */}
                <div className="col-span-12 lg:col-span-6 glass-card rounded-[2.5rem] p-10 shadow-sm">
                    <h4 className="font-bold text-slate-800 mb-10 text-left uppercase tracking-[0.1em]">การใช้บริการแยกรายคณะ (By Department)</h4>
                    <div className="space-y-6">
                        {[
                            { n: 'วิศวกรรมศาสตร์และนวัตกรรม', u: '12,450', p: 92 },
                            { n: 'บริหารธุรกิจและบัญชี', u: '8,120', p: 65 },
                            { n: 'ศิลปกรรมและดีไซน์', u: '5,900', p: 48 },
                            { n: 'วิทยาศาสตร์และแพทย์', u: '4,210', p: 32 }
                        ].map((dept, i) => (
                            <div key={i} className="space-y-2 text-left">
                                <div className="flex justify-between items-center text-[11px] font-bold uppercase tracking-tight">
                                    <span className="text-slate-700">{dept.n}</span>
                                    <span className="text-[#7c3aed]">{dept.u} เซสชัน</span>
                                </div>
                                <div className="h-2 w-full bg-slate-100 rounded-full overflow-hidden shadow-inner">
                                    <div className="h-full bg-gradient-to-r from-purple-400 to-[#7c3aed] rounded-full transition-all duration-1000" style={{ width: `${dept.p}%` }}></div>
                                </div>
                            </div>
                        ))}
                    </div>
                </div>

                {/* 6. Peak Time Intensity Map */}
                <div className="col-span-12 lg:col-span-6 glass-card rounded-[2.5rem] p-10 shadow-sm text-left">
                    <div className="flex items-center justify-between mb-10">
                        <h4 className="font-bold text-slate-800 uppercase tracking-widest">ฮีตแมปช่วงเวลา (Peak Time Map)</h4>
                        <span className="text-[9px] font-bold text-slate-400 bg-slate-50 px-2.5 py-1 rounded-lg border border-slate-100 uppercase">อิงจากค่าเฉลี่ยรายปี</span>
                    </div>
                    <div className="grid grid-cols-12 gap-2 items-end" style={{ height: '176px' }}>
                        {[12, 18, 35, 60, 85, 98, 100, 88, 70, 45, 25, 10].map((h, i) => (
                            <div
                                key={i}
                                className={`rounded-t-lg transition-all duration-500 ${h > 90 ? 'opacity-100 scale-x-110 shadow-lg shadow-purple-100' : 'opacity-60 hover:opacity-100'}`}
                                style={{ height: `${h}%`, background: 'linear-gradient(to top, #ede9fe, #7c3aed)' }}
                            ></div>
                        ))}
                    </div>
                    <div className="flex justify-between mt-4 text-[10px] font-bold text-slate-400 uppercase tracking-widest">
                        <span>08:00 น.</span>
                        <span className="text-[#7c3aed] font-black underline underline-offset-4">13:30 น. - ช่วงพีควิกฤต</span>
                        <span>20:00 น.</span>
                    </div>
                    <div className="mt-8 flex items-center gap-4 bg-purple-50/50 border border-purple-100 p-5 rounded-2xl">
                        <div className="w-10 h-10 rounded-2xl bg-white flex items-center justify-center shrink-0 shadow-sm">
                            <span className="material-symbols-outlined text-[#7c3aed] text-xl">lightbulb</span>
                        </div>
                        <p className="text-[11px] text-slate-600 font-medium leading-relaxed italic">
                            ข้อเสนอแนะ: การใช้งานเกินเกณฑ์วิกฤต <span className="font-black text-[#7c3aed]">90%</span> ระหว่าง 11:45 - 14:30 เป็นประจำ แนะนำให้พิจารณาขยายพื้นที่สำหรับเทอมหน้า
                        </p>
                    </div>
                </div>

                {/* 7. Session Health & Integrity */}
                <div className="col-span-12 glass-card rounded-[2.5rem] p-10 shadow-sm flex flex-col md:flex-row items-center justify-between border-l-4 border-l-emerald-400 gap-8">
                    <div className="text-left w-full md:w-auto">
                        <h4 className="font-bold text-slate-800 text-lg">สุขภาพของเซสชัน (Session Health)</h4>
                        <p className="text-xs text-slate-500 mt-1">สัดส่วนผู้ใช้ที่ล็อกเอาต์ปกติ vs ถูกบังคับเซสชัน vs ระบบขัดข้อง</p>
                    </div>
                    <div className="flex gap-12 items-center flex-1 justify-center w-full md:w-auto">
                        <div className="text-center">
                            <p className="text-[10px] font-bold text-emerald-500 uppercase tracking-widest mb-1">ปกติ (Normal)</p>
                            <h3 className="text-3xl font-black text-slate-800">98.2%</h3>
                        </div>
                        <div className="text-center border-l border-slate-100 pl-12 hidden sm:block">
                            <p className="text-[10px] font-bold text-rose-500 uppercase tracking-widest mb-1">ตัดจบ (Terminated)</p>
                            <h3 className="text-3xl font-black text-slate-800">1.5%</h3>
                        </div>
                        <div className="text-center border-l border-slate-100 pl-12 hidden sm:block">
                            <p className="text-[10px] font-bold text-amber-500 uppercase tracking-widest mb-1">ขัดข้อง (Errors)</p>
                            <h3 className="text-3xl font-black text-slate-800">0.3%</h3>
                        </div>
                    </div>
                    <div className="w-full md:w-1/3 h-3 bg-slate-100 rounded-full flex overflow-hidden shadow-inner">
                        <div className="h-full bg-emerald-400" style={{width: '98.2%'}} title="Normal: 98.2%"></div>
                        <div className="h-full bg-rose-400" style={{width: '1.5%'}} title="Terminated: 1.5%"></div>
                        <div className="h-full bg-amber-400" style={{width: '0.3%'}} title="Errors: 0.3%"></div>
                    </div>
                </div>
            </div>
        </div>
    );
}

export default ExecOverview;
