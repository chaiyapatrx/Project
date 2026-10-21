import { APP_NAME } from './brand';

// Product name with an optional role label, used in every page header.
export default function BrandName({ role }) {
    return (
        <span className="flex flex-col leading-tight">
            <span className="font-extrabold text-base text-slate-800">{APP_NAME}</span>
            {role && <span className="text-xs font-bold text-[#7c3aed]">{role}</span>}
        </span>
    );
}
