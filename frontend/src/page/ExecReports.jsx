import React, { useState } from 'react';

function ExecReports() {
    const [reportTab, setReportTab] = useState('bookings');

    // Mock data for booking history
    const bookingHistory = [
        { id: 1, user: 'สมชาย ใจดี', username: 'somchai01', computer: 'COM-05', date: '24 มี.ค. 2568', timeStart: '09:00', timeEnd: '11:30', duration: '2h 30m', status: 'completed', dept: 'วิศวกรรมศาสตร์' },
        { id: 2, user: 'สุภาพร แสงทอง', username: 'supaporn02', computer: 'COM-12', date: '24 มี.ค. 2568', timeStart: '10:15', timeEnd: '12:00', duration: '1h 45m', status: 'completed', dept: 'บริหารธุรกิจ' },
        { id: 3, user: 'วิชัย พานทอง', username: 'wichai03', computer: 'COM-21', date: '24 มี.ค. 2568', timeStart: '13:00', timeEnd: '-', duration: 'กำลังใช้งาน', status: 'active', dept: 'วิทยาศาสตร์' },
        { id: 4, user: 'นิรันดร์ สุขสมบูรณ์', username: 'nirun04', computer: 'COM-03', date: '23 มี.ค. 2568', timeStart: '14:00', timeEnd: '14:05', duration: '5m', status: 'terminated', dept: 'ศิลปกรรม' },
        { id: 5, user: 'พิมลพรรณ ดาวเรือง', username: 'pimon05', computer: 'COM-48', date: '23 มี.ค. 2568', timeStart: '08:30', timeEnd: '10:45', duration: '2h 15m', status: 'completed', dept: 'วิศวกรรมศาสตร์' },
        { id: 6, user: 'ธนพล ศรีสุข', username: 'thanapol06', computer: 'COM-11', date: '23 มี.ค. 2568', timeStart: '15:30', timeEnd: '17:00', duration: '1h 30m', status: 'completed', dept: 'บริหารธุรกิจ' },
        { id: 7, user: 'จิราภรณ์ วงศ์ดี', username: 'jiraporn07', computer: 'COM-33', date: '22 มี.ค. 2568', timeStart: '09:45', timeEnd: '12:15', duration: '2h 30m', status: 'completed', dept: 'วิทยาศาสตร์' },
        { id: 8, user: 'กฤษฎา รัตนพงษ์', username: 'kritsada08', computer: 'COM-07', date: '22 มี.ค. 2568', timeStart: '13:00', timeEnd: '13:02', duration: '2m', status: 'error', dept: 'วิศวกรรมศาสตร์' },
    ];

    // Mock weekly summary
    const weeklySummary = [
        { day: 'จันทร์', sessions: 245, avgDuration: '1h 42m', peak: '13:00-14:00' },
        { day: 'อังคาร', sessions: 312, avgDuration: '1h 55m', peak: '10:00-11:00' },
        { day: 'พุธ', sessions: 198, avgDuration: '1h 20m', peak: '13:00-14:00' },
        { day: 'พฤหัสบดี', sessions: 287, avgDuration: '1h 48m', peak: '14:00-15:00' },
        { day: 'ศุกร์', sessions: 351, avgDuration: '2h 05m', peak: '10:00-11:00' },
        { day: 'เสาร์', sessions: 89, avgDuration: '2h 30m', peak: '11:00-12:00' },
        { day: 'อาทิตย์', sessions: 42, avgDuration: '1h 15m', peak: '14:00-15:00' },
    ];

    const statusBadge = (status) => {
        const map = {
            completed: { bg: 'bg-emerald-50 text-emerald-600 border-emerald-200', label: 'สำเร็จ' },
            active: { bg: 'bg-blue-50 text-blue-600 border-blue-200', label: 'กำลังใช้งาน' },
            terminated: { bg: 'bg-rose-50 text-rose-600 border-rose-200', label: 'ถูกตัดจบ' },
            error: { bg: 'bg-amber-50 text-amber-600 border-amber-200', label: 'ระบบขัดข้อง' },
        };
        const s = map[status] || map.completed;
        return <span className={`px-2 py-0.5 rounded-full border text-[9px] font-bold uppercase tracking-wider ${s.bg}`}>{s.label}</span>;
    };

    return (
        <div className="p-8 space-y-8 h-full overflow-y-auto custom-scrollbar">
            {/* Report Tab Switcher */}
            <div className="flex items-center gap-2 bg-slate-100 p-1 rounded-xl w-fit">
                <button
                    onClick={() => setReportTab('bookings')}
                    className={`px-5 py-2.5 rounded-lg text-xs font-bold transition-all ${reportTab === 'bookings' ? 'bg-white text-[#7c3aed] shadow-sm' : 'text-slate-500 hover:text-slate-700'}`}
                >
                    <span className="material-symbols-outlined text-sm align-middle mr-1">history</span>
                    ประวัติการจอง (Booking History)
                </button>
                <button
                    onClick={() => setReportTab('weekly')}
                    className={`px-5 py-2.5 rounded-lg text-xs font-bold transition-all ${reportTab === 'weekly' ? 'bg-white text-[#7c3aed] shadow-sm' : 'text-slate-500 hover:text-slate-700'}`}
                >
                    <span className="material-symbols-outlined text-sm align-middle mr-1">calendar_view_week</span>
                    สรุปรายสัปดาห์ (Weekly Summary)
                </button>
                <button
                    onClick={() => setReportTab('machines')}
                    className={`px-5 py-2.5 rounded-lg text-xs font-bold transition-all ${reportTab === 'machines' ? 'bg-white text-[#7c3aed] shadow-sm' : 'text-slate-500 hover:text-slate-700'}`}
                >
                    <span className="material-symbols-outlined text-sm align-middle mr-1">desktop_windows</span>
                    สถิติรายเครื่อง (Machine Stats)
                </button>
            </div>

            {/* ==== TAB 1: Booking History ==== */}
            {reportTab === 'bookings' && (
                <div className="glass-card rounded-2xl border border-slate-100 overflow-hidden shadow-sm">
                    {/* Summary Strip */}
                    <div className="grid grid-cols-4 gap-0 border-b border-slate-100">
                        {[
                            { label: 'การจองทั้งหมด', value: '1,524', icon: 'bookmark_added', color: 'text-[#7c3aed]' },
                            { label: 'สำเร็จ', value: '1,492', icon: 'check_circle', color: 'text-emerald-500' },
                            { label: 'ถูกตัดจบ', value: '23', icon: 'cancel', color: 'text-rose-500' },
                            { label: 'ระบบขัดข้อง', value: '9', icon: 'error', color: 'text-amber-500' },
                        ].map((s, i) => (
                            <div key={i} className={`p-5 text-center ${i < 3 ? 'border-r border-slate-100' : ''}`}>
                                <span className={`material-symbols-outlined text-2xl ${s.color}`}>{s.icon}</span>
                                <h3 className="text-2xl font-black text-slate-800 mt-1">{s.value}</h3>
                                <p className="text-[10px] font-bold text-slate-400 uppercase tracking-widest mt-1">{s.label}</p>
                            </div>
                        ))}
                    </div>

                    {/* Table */}
                    <table className="w-full text-left border-collapse">
                        <thead>
                            <tr className="bg-slate-50/50 border-b border-slate-100 text-xs font-bold text-slate-500 uppercase tracking-wider">
                                <th className="p-4 pl-6">ผู้ใช้งาน</th>
                                <th className="p-4">คณะ/แผนก</th>
                                <th className="p-4">เครื่อง</th>
                                <th className="p-4">วันที่</th>
                                <th className="p-4">เวลาเริ่ม-สิ้นสุด</th>
                                <th className="p-4">ระยะเวลา</th>
                                <th className="p-4 pr-6">สถานะ</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-slate-100 bg-white/50">
                            {bookingHistory.map((b) => (
                                <tr key={b.id} className="hover:bg-purple-50/30 transition-colors">
                                    <td className="p-4 pl-6">
                                        <div className="flex items-center gap-3">
                                            <div className="w-8 h-8 rounded-full bg-purple-100 flex items-center justify-center text-[#7c3aed] font-bold text-[10px] uppercase">
                                                {b.user.substring(0, 2)}
                                            </div>
                                            <div>
                                                <p className="font-bold text-slate-700 text-sm">{b.user}</p>
                                                <p className="text-[10px] text-slate-400">@{b.username}</p>
                                            </div>
                                        </div>
                                    </td>
                                    <td className="p-4 text-xs text-slate-600 font-medium">{b.dept}</td>
                                    <td className="p-4">
                                        <span className="bg-slate-100 text-slate-700 px-2 py-1 rounded text-[10px] font-bold">{b.computer}</span>
                                    </td>
                                    <td className="p-4 text-xs text-slate-600">{b.date}</td>
                                    <td className="p-4 text-xs text-slate-600 font-mono">{b.timeStart} - {b.timeEnd}</td>
                                    <td className="p-4 text-xs font-bold text-slate-700">{b.duration}</td>
                                    <td className="p-4 pr-6">{statusBadge(b.status)}</td>
                                </tr>
                            ))}
                        </tbody>
                    </table>

                    {/* Footer */}
                    <div className="p-4 border-t border-slate-100 bg-slate-50/50 text-xs text-slate-500 flex items-center justify-between font-medium">
                        <span>แสดง {bookingHistory.length} รายการล่าสุด</span>
                        <div className="flex gap-1">
                            <button className="px-3 py-1 rounded bg-white border border-slate-200 hover:border-purple-300 hover:text-purple-600 transition-colors">ก่อนหน้า</button>
                            <button className="px-3 py-1 rounded bg-[#7c3aed] text-white shadow-sm shadow-purple-200">1</button>
                            <button className="px-3 py-1 rounded bg-white border border-slate-200 hover:border-purple-300 hover:text-purple-600 transition-colors">2</button>
                            <button className="px-3 py-1 rounded bg-white border border-slate-200 hover:border-purple-300 hover:text-purple-600 transition-colors">ถัดไป</button>
                        </div>
                    </div>
                </div>
            )}

            {/* ==== TAB 2: Weekly Summary ==== */}
            {reportTab === 'weekly' && (
                <div className="space-y-8">
                    {/* Quick Stat Cards */}
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                        {[
                            { label: 'จำนวนเซสชันรวม (สัปดาห์นี้)', value: '1,524', sub: '+8.3% จากสัปดาห์ก่อน', icon: 'trending_up', color: 'from-purple-500 to-[#7c3aed]' },
                            { label: 'ระยะเวลาเฉลี่ย/เซสชัน', value: '1h 48m', sub: 'เพิ่มขึ้น 12 นาที', icon: 'schedule', color: 'from-blue-500 to-blue-600' },
                            { label: 'เครื่องที่ใช้งานมากที่สุด', value: 'COM-12', sub: '189 เซสชัน/สัปดาห์', icon: 'star', color: 'from-amber-400 to-amber-500' },
                        ].map((card, i) => (
                            <div key={i} className="glass-card rounded-2xl p-6 shadow-sm relative overflow-hidden">
                                <div className={`absolute top-0 right-0 w-24 h-24 bg-gradient-to-br ${card.color} opacity-5 rounded-full -mr-8 -mt-8`}></div>
                                <div className="flex items-center gap-3 mb-4">
                                    <div className={`w-10 h-10 rounded-xl bg-gradient-to-br ${card.color} flex items-center justify-center shadow-lg`}>
                                        <span className="material-symbols-outlined text-white text-lg">{card.icon}</span>
                                    </div>
                                    <span className="text-[10px] font-bold text-slate-400 uppercase tracking-widest flex-1">{card.label}</span>
                                </div>
                                <h3 className="text-2xl font-black text-slate-800">{card.value}</h3>
                                <p className="text-[11px] font-bold text-emerald-500 mt-1 flex items-center gap-1">
                                    <span className="w-1.5 h-1.5 rounded-full bg-emerald-500"></span> {card.sub}
                                </p>
                            </div>
                        ))}
                    </div>

                    {/* Weekly Table */}
                    <div className="glass-card rounded-2xl border border-slate-100 overflow-hidden shadow-sm">
                        <div className="p-6 border-b border-slate-100">
                            <h4 className="font-bold text-slate-800">สรุปรายวันในสัปดาห์นี้ (Daily Breakdown)</h4>
                            <p className="text-xs text-slate-400 mt-1">ข้อมูลสำหรับสัปดาห์ที่ 18-24 มีนาคม 2568</p>
                        </div>
                        <table className="w-full text-left border-collapse">
                            <thead>
                                <tr className="bg-slate-50/50 border-b border-slate-100 text-xs font-bold text-slate-500 uppercase tracking-wider">
                                    <th className="p-4 pl-6">วัน</th>
                                    <th className="p-4">จำนวนเซสชัน</th>
                                    <th className="p-4">ค่าเฉลี่ย/เซสชัน</th>
                                    <th className="p-4">ช่วง Peak</th>
                                    <th className="p-4 pr-6">ระดับคิว</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-slate-100 bg-white/50">
                                {weeklySummary.map((row, i) => {
                                    const peakLevel = row.sessions > 300 ? 'สูงมาก' : row.sessions > 200 ? 'ปานกลาง' : 'ต่ำ';
                                    const peakColor = row.sessions > 300 ? 'bg-rose-50 text-rose-600 border-rose-200' : row.sessions > 200 ? 'bg-amber-50 text-amber-600 border-amber-200' : 'bg-emerald-50 text-emerald-600 border-emerald-200';
                                    return (
                                        <tr key={i} className="hover:bg-purple-50/30 transition-colors">
                                            <td className="p-4 pl-6 font-bold text-slate-700 text-sm">{row.day}</td>
                                            <td className="p-4">
                                                <div className="flex items-center gap-3">
                                                    <span className="text-sm font-black text-slate-800">{row.sessions}</span>
                                                    <div className="w-24 h-1.5 bg-slate-100 rounded-full overflow-hidden">
                                                        <div className="h-full bg-gradient-to-r from-purple-400 to-[#7c3aed] rounded-full" style={{ width: `${(row.sessions / 351) * 100}%` }}></div>
                                                    </div>
                                                </div>
                                            </td>
                                            <td className="p-4 text-xs text-slate-600 font-mono">{row.avgDuration}</td>
                                            <td className="p-4 text-xs text-slate-600">{row.peak}</td>
                                            <td className="p-4 pr-6">
                                                <span className={`px-2 py-0.5 rounded-full border text-[9px] font-bold uppercase tracking-wider ${peakColor}`}>{peakLevel}</span>
                                            </td>
                                        </tr>
                                    );
                                })}
                            </tbody>
                        </table>
                    </div>
                </div>
            )}

            {/* ==== TAB 3: Machine Stats ==== */}
            {reportTab === 'machines' && (
                <div className="space-y-8">
                    {/* Top Machines */}
                    <div className="glass-card rounded-2xl p-8 shadow-sm border border-slate-100">
                        <h4 className="font-bold text-slate-800 mb-8">เครื่องที่มีการใช้งานมากที่สุด (Top 10 Most Used)</h4>
                        <div className="space-y-4">
                            {[
                                { name: 'COM-12', sessions: 189, hours: 312, pct: 100 },
                                { name: 'COM-05', sessions: 175, hours: 289, pct: 93 },
                                { name: 'COM-21', sessions: 168, hours: 272, pct: 89 },
                                { name: 'COM-33', sessions: 152, hours: 248, pct: 80 },
                                { name: 'COM-48', sessions: 141, hours: 231, pct: 75 },
                                { name: 'COM-03', sessions: 134, hours: 215, pct: 71 },
                                { name: 'COM-07', sessions: 128, hours: 198, pct: 68 },
                                { name: 'COM-15', sessions: 115, hours: 182, pct: 61 },
                                { name: 'COM-29', sessions: 98, hours: 156, pct: 52 },
                                { name: 'COM-41', sessions: 87, hours: 134, pct: 46 },
                            ].map((m, i) => (
                                <div key={i} className="flex items-center gap-4">
                                    <span className={`w-7 h-7 rounded-lg flex items-center justify-center text-xs font-black ${i < 3 ? 'bg-[#7c3aed] text-white' : 'bg-slate-100 text-slate-500'}`}>{i + 1}</span>
                                    <span className="font-bold text-slate-700 text-sm w-20">{m.name}</span>
                                    <div className="flex-1">
                                        <div className="h-2.5 bg-slate-100 rounded-full overflow-hidden">
                                            <div className={`h-full rounded-full transition-all duration-1000 ${i < 3 ? 'bg-gradient-to-r from-purple-400 to-[#7c3aed]' : 'bg-slate-300'}`} style={{ width: `${m.pct}%` }}></div>
                                        </div>
                                    </div>
                                    <span className="text-xs font-bold text-[#7c3aed] w-24 text-right">{m.sessions} เซสชัน</span>
                                    <span className="text-xs font-medium text-slate-400 w-24 text-right">{m.hours} ชม. รวม</span>
                                </div>
                            ))}
                        </div>
                    </div>

                    {/* Machine Health */}
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
                        <div className="glass-card rounded-2xl p-8 shadow-sm border border-slate-100">
                            <h4 className="font-bold text-slate-800 mb-6">สถานะเครื่องปัจจุบัน (Current Status)</h4>
                            <div className="grid grid-cols-2 gap-6">
                                {[
                                    { label: 'ออนไลน์ (Online)', value: '42', icon: 'wifi', color: 'text-emerald-500 bg-emerald-50' },
                                    { label: 'ออฟไลน์ (Offline)', value: '3', icon: 'wifi_off', color: 'text-slate-400 bg-slate-50' },
                                    { label: 'กำลังใช้งาน (In Use)', value: '28', icon: 'person', color: 'text-blue-500 bg-blue-50' },
                                    { label: 'ซ่อมบำรุง (Maintenance)', value: '5', icon: 'build', color: 'text-orange-500 bg-orange-50' },
                                ].map((item, i) => (
                                    <div key={i} className="flex items-center gap-3 p-4 rounded-xl bg-slate-50/50 border border-slate-100">
                                        <div className={`w-10 h-10 rounded-xl flex items-center justify-center ${item.color}`}>
                                            <span className="material-symbols-outlined text-xl">{item.icon}</span>
                                        </div>
                                        <div>
                                            <h3 className="text-xl font-black text-slate-800">{item.value}</h3>
                                            <p className="text-[9px] font-bold text-slate-400 uppercase tracking-widest">{item.label}</p>
                                        </div>
                                    </div>
                                ))}
                            </div>
                        </div>

                        <div className="glass-card rounded-2xl p-8 shadow-sm border border-slate-100">
                            <h4 className="font-bold text-slate-800 mb-6">เครื่องที่ต้องตรวจสอบ (Needs Attention)</h4>
                            <div className="space-y-3">
                                {[
                                    { name: 'COM-18', issue: 'ออฟไลน์มากกว่า 24 ชม.', severity: 'สูง', color: 'border-l-rose-400 bg-rose-50/30' },
                                    { name: 'COM-36', issue: 'อัตราข้อผิดพลาดสูง (3.2%)', severity: 'ปานกลาง', color: 'border-l-amber-400 bg-amber-50/30' },
                                    { name: 'COM-44', issue: 'Heartbeat ขาดหาย', severity: 'สูง', color: 'border-l-rose-400 bg-rose-50/30' },
                                    { name: 'COM-09', issue: 'เข้าสู่โหมดซ่อมบำรุง 3 วันแล้ว', severity: 'ต่ำ', color: 'border-l-blue-400 bg-blue-50/30' },
                                ].map((alert, i) => (
                                    <div key={i} className={`p-4 border-l-4 rounded-xl flex items-center justify-between ${alert.color}`}>
                                        <div className="flex items-center gap-3">
                                            <span className="material-symbols-outlined text-lg text-slate-500">desktop_windows</span>
                                            <div>
                                                <p className="text-sm font-bold text-slate-700">{alert.name}</p>
                                                <p className="text-[10px] text-slate-500">{alert.issue}</p>
                                            </div>
                                        </div>
                                        <span className={`text-[9px] font-bold uppercase tracking-wider px-2 py-0.5 rounded-full border ${alert.severity === 'สูง' ? 'text-rose-600 bg-rose-50 border-rose-200' :
                                                alert.severity === 'ปานกลาง' ? 'text-amber-600 bg-amber-50 border-amber-200' :
                                                    'text-blue-600 bg-blue-50 border-blue-200'
                                            }`}>{alert.severity}</span>
                                    </div>
                                ))}
                            </div>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}

export default ExecReports;
