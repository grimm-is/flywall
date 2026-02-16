export function formatBytes(bytes: number, decimals: number = 2): string {
    if (bytes === 0) return '0 B';

    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];

    const i = Math.floor(Math.log(bytes) / Math.log(k));

    return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}

export function formatDuration(seconds: number): string {
    if (seconds < 60) return `${seconds}s`;

    const minutes = Math.floor(seconds / 60);
    const secs = seconds % 60;

    if (minutes < 60) return `${minutes}m ${secs}s`;

    const hours = Math.floor(minutes / 60);
    const mins = minutes % 60;

    if (hours < 24) return `${hours}h ${mins}m`;

    const days = Math.floor(hours / 24);
    const hrs = hours % 24;

    return `${days}d ${hrs}h`;
}
