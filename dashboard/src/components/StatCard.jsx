export default function StatCard({ title, value, subtitle, icon, iconColor = 'blue' }) {
  const colorMap = {
    blue: 'bg-blue-50 text-blue-600 border-blue-100',
    indigo: 'bg-indigo-50 text-indigo-600 border-indigo-100',
    emerald: 'bg-emerald-50 text-emerald-600 border-emerald-100',
    amber: 'bg-amber-50 text-amber-600 border-amber-100',
    purple: 'bg-purple-50 text-purple-600 border-purple-100',
  };

  const badgeStyle = colorMap[iconColor] || colorMap.blue;

  return (
    <div className="bg-white rounded-xl border border-slate-200/80 p-5 shadow-xs hover:border-slate-300 transition-all">
      <div className="flex items-center justify-between">
        <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">{title}</p>
        {icon && (
          <div className={`w-9 h-9 rounded-lg border flex items-center justify-center ${badgeStyle}`}>
            {icon}
          </div>
        )}
      </div>
      <div className="mt-3">
        <p className="text-2xl font-bold text-slate-900 tracking-tight">{value}</p>
        {subtitle && (
          <p className="mt-1 text-xs text-slate-500 font-medium flex items-center gap-1">
            {subtitle}
          </p>
        )}
      </div>
    </div>
  );
}
