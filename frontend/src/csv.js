export function csvCell(value) {
    let text = String(value ?? '');
    let first = 0;
    while (first < text.length && text.charCodeAt(first) <= 0x20) first += 1;
    if (['=', '+', '-', '@'].includes(text[first])) text = `'${text}`;
    return `"${text.replaceAll('"', '""')}"`;
}

export function toCSV(headers, rows) {
    return [headers, ...rows].map(row => row.map(csvCell).join(',')).join('\r\n') + '\r\n';
}
