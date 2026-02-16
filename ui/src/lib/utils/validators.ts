export const validators = {
    // Matches standard Linux interface names (eth0, ens3) + wildcards (* or + at end)
    interfaceName: /^[a-zA-Z0-9_.-]+(?:[*+]?)$/,

    // Simple Identifier (alphanumeric + underscore)
    identifier: /^[a-zA-Z0-9_]+$/,

    // Hostname (RFC 1123)
    hostname: /^(?![0-9]+$)(?!-)[a-zA-Z0-9-]{1,63}(?<!-)(?:\.[a-zA-Z0-9-]{1,63})*$/,

    // IPv4 CIDR or Address
    ipv4Cidr: /^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(?:\/(3[0-2]|[12]?[0-9]))?$/,

    // IPv6 CIDR or Address (Simple regex, non-exhaustive but pragmatic)
    ipv6Cidr: /^([0-9a-fA-F]{1,4}:){1,7}:?([0-9a-fA-F]{1,4})?(?:\/(12[0-8]|1[0-1][0-9]|[1-9][0-9]?))?$/,

    // MAC Address (00:11:22:aa:bb:cc or 00-11-22-aa-bb-cc)
    macAddress: /^([0-9a-fA-F]{2}[:-]){5}([0-9a-fA-F]{2})$/,

    // Protocol (tcp, udp, icmp, gre, esp, ah, or number 0-255)
    // Note: complex protocols like "icmpv6" also valid
    protocol: /^([a-zA-Z0-9]+)$/,

    // integer/range (0-65535 or range 10-20)
    integerOrRange: /^(\d+)(?:-(\d+))?$/,

    // Hex check for Mark (0x123 or 123)
    hexOrInt: /^(?:0x[0-9a-fA-F]+|\d+)$/
};

export function isValidSelector(type: string, value: string): { valid: boolean; error?: string } {
    if (!value) return { valid: false, error: "Value is required" };

    switch (type) {
        case "interface":
            if (!validators.interfaceName.test(value)) {
                return { valid: false, error: "Invalid interface name format (start with alphanumeric, can end with * or +)" };
            }
            break;
        case "cidr":
        case "src":
        case "dst":
            if (!validators.ipv4Cidr.test(value) && !validators.ipv6Cidr.test(value)) {
                return { valid: false, error: "Invalid IP address or CIDR notation" };
            }
            break;
        case "mac":
            if (!validators.macAddress.test(value)) {
                return { valid: false, error: "Invalid MAC address (e.g. 00:11:22:aa:bb:cc)" };
            }
            break;
        case "protocol":
            // Allow names (tcp, udp) or numbers (1-255)
            if (!validators.identifier.test(value)) {
                return { valid: false, error: "Invalid protocol name or number" };
            }
            break;
        case "mark":
        case "dscp":
        case "tos":
            if (!validators.hexOrInt.test(value)) {
                return { valid: false, error: "Must be a number or hex value (0x...)" };
            }
            break;
        case "ipset":
            if (!validators.identifier.test(value)) {
                return { valid: false, error: "Invalid IPSet name (alphanumeric and underscores only)" };
            }
            break;
        case "hostname":
            if (!validators.hostname.test(value)) {
                return { valid: false, error: "Invalid hostname format" };
            }
            break;
        case "rule":
            // "Rule" matches raw config snippets often. For now allow broadly but prevent dangerous chars if needed.
            // HCL string literal basics.
            if (value.includes('"')) {
                return { valid: false, error: "Rule cannot contain double quotes" };
            }
            break;
    }
    return { valid: true };
}
