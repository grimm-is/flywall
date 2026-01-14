/**
 * Flows Store Tests
 * Tests for the flows/conntrack store
 */

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { get } from "svelte/store";
import { flowsStore, formatBytes, formatAge, formatRate } from "$lib/stores/flows";

// Mock fetch
const mockFetch = vi.fn();
vi.stubGlobal("fetch", mockFetch);

describe("Flows Store", () => {
    beforeEach(() => {
        flowsStore.reset();
        mockFetch.mockReset();
    });

    afterEach(() => {
        flowsStore.stopPolling();
    });

    describe("initial state", () => {
        it("should have empty flows array", () => {
            const state = get(flowsStore);
            expect(state.flows).toEqual([]);
        });

        it("should not be loading initially", () => {
            const state = get(flowsStore);
            expect(state.loading).toBe(false);
        });

        it("should have no error initially", () => {
            const state = get(flowsStore);
            expect(state.error).toBeNull();
        });
    });

    describe("fetch", () => {
        it("should fetch flows from API", async () => {
            const mockFlows = [
                { id: "1", src_ip: "192.168.1.10", dst_ip: "8.8.8.8", protocol: "tcp" },
                { id: "2", src_ip: "192.168.1.11", dst_ip: "1.1.1.1", protocol: "udp" },
            ];

            mockFetch.mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve(mockFlows),
            });

            await flowsStore.fetch();

            const state = get(flowsStore);
            expect(state.flows).toHaveLength(2);
            expect(state.flows[0].id).toBe("1");
            expect(state.loading).toBe(false);
            expect(state.lastUpdate).toBeInstanceOf(Date);
        });

        it("should handle fetch error", async () => {
            mockFetch.mockResolvedValueOnce({
                ok: false,
                status: 500,
            });

            await expect(flowsStore.fetch()).rejects.toThrow("HTTP 500");

            const state = get(flowsStore);
            expect(state.error).toBe("HTTP 500");
            expect(state.loading).toBe(false);
        });

        it("should handle array in flows field", async () => {
            mockFetch.mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve({ flows: [{ id: "1" }] }),
            });

            await flowsStore.fetch();

            const state = get(flowsStore);
            expect(state.flows).toHaveLength(1);
        });
    });

    describe("kill", () => {
        it("should remove flow from local state on success", async () => {
            // Set initial flows
            mockFetch.mockResolvedValueOnce({
                ok: true,
                json: () =>
                    Promise.resolve([
                        { id: "1", src_ip: "192.168.1.10" },
                        { id: "2", src_ip: "192.168.1.11" },
                    ]),
            });
            await flowsStore.fetch();

            // Mock kill request
            mockFetch.mockResolvedValueOnce({ ok: true });
            await flowsStore.kill("1");

            const state = get(flowsStore);
            expect(state.flows).toHaveLength(1);
            expect(state.flows[0].id).toBe("2");
        });

        it("should throw on kill failure", async () => {
            mockFetch.mockResolvedValueOnce({
                ok: false,
                status: 404,
            });

            await expect(flowsStore.kill("999")).rejects.toThrow("HTTP 404");
        });
    });
});

describe("Flow formatters", () => {
    describe("formatBytes", () => {
        it("should format bytes", () => {
            expect(formatBytes(0)).toBe("0 B");
            expect(formatBytes(512)).toBe("512 B");
        });

        it("should format kilobytes", () => {
            expect(formatBytes(1024)).toBe("1.0 KB");
            expect(formatBytes(1536)).toBe("1.5 KB");
        });

        it("should format megabytes", () => {
            expect(formatBytes(1024 * 1024)).toBe("1.0 MB");
            expect(formatBytes(5.5 * 1024 * 1024)).toBe("5.5 MB");
        });

        it("should format gigabytes", () => {
            expect(formatBytes(1024 * 1024 * 1024)).toBe("1.00 GB");
            expect(formatBytes(2.5 * 1024 * 1024 * 1024)).toBe("2.50 GB");
        });
    });

    describe("formatAge", () => {
        it("should format seconds", () => {
            expect(formatAge(0)).toBe("0s");
            expect(formatAge(30)).toBe("30s");
            expect(formatAge(59)).toBe("59s");
        });

        it("should format minutes", () => {
            expect(formatAge(60)).toBe("1m");
            expect(formatAge(120)).toBe("2m");
            expect(formatAge(3599)).toBe("59m");
        });

        it("should format hours", () => {
            expect(formatAge(3600)).toBe("1h");
            expect(formatAge(7200)).toBe("2h");
            expect(formatAge(86399)).toBe("23h");
        });

        it("should format days", () => {
            expect(formatAge(86400)).toBe("1d");
            expect(formatAge(172800)).toBe("2d");
        });
    });

    describe("formatRate", () => {
        it("should calculate rate from bytes and time", () => {
            expect(formatRate(1024, 1)).toBe("1.0 KB/s");
            expect(formatRate(1024 * 1024, 1)).toBe("1.0 MB/s");
            expect(formatRate(1024 * 1024, 2)).toBe("512.0 KB/s");
        });

        it("should handle zero time", () => {
            expect(formatRate(1024, 0)).toBe("0 B/s");
        });

        it("should handle negative time", () => {
            expect(formatRate(1024, -1)).toBe("0 B/s");
        });
    });
});
