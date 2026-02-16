// @ts-nocheck
import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import ZoneSelectorEditor from './ZoneSelectorEditor.svelte';

describe('ZoneSelectorEditor', () => {
    it('renders empty state initially', () => {
        render(ZoneSelectorEditor, { matches: [], availableInterfaces: [], availableIPSets: [] });
        expect(screen.getByText(/No match rules defined/i)).toBeTruthy();
        expect(screen.getByText('Add Match Rule')).toBeTruthy();
    });

    it('adds a new match rule', async () => {
        render(ZoneSelectorEditor, { matches: [], availableInterfaces: [], availableIPSets: [] });
        const button = screen.getByText('Add Match Rule');
        await fireEvent.click(button);

        expect(screen.getByText('New Match Rule')).toBeTruthy();
    });

    it('shows advanced options when toggled', async () => {
        render(ZoneSelectorEditor, { matches: [], availableInterfaces: [], availableIPSets: [] });
        await fireEvent.click(screen.getByText('Add Match Rule'));

        const toggle = screen.getByText('Show Advanced Options');
        expect(toggle).toBeTruthy();

        await fireEvent.click(toggle);

        expect(screen.getByText('Hide Advanced')).toBeTruthy();
        // Check for specific advanced fields by placeholder or label
        expect(screen.getByLabelText('Protocol')).toBeTruthy();
        expect(screen.getByLabelText('MAC Address')).toBeTruthy();
    });

    it('saves a new rule', async () => {
        let capturedMatches: any[] = [];
        const onchange = (e: any) => {
            capturedMatches = e.detail;
        };

        // @ts-ignore
        render(ZoneSelectorEditor, {
            matches: [],
            availableInterfaces: [],
            availableIPSets: [],
            onchange
        });

        await fireEvent.click(screen.getByText('Add Match Rule'));

        // Fill Interface
        const input = screen.getByPlaceholderText('e.g. eth0');
        await fireEvent.input(input, { target: { value: 'eth0' } });

        // Save
        const saveBtn = screen.getByText('Add Rule');
        await fireEvent.click(saveBtn);

        expect(capturedMatches.length).toBe(1);
        expect(capturedMatches[0].interface).toBe('eth0');
    });

    it('renders existing matches correctly', () => {
        const matches = [
            { _id: '1', interface: 'eth0', protocol: 'tcp' }
        ];
        render(ZoneSelectorEditor, { matches, availableInterfaces: [], availableIPSets: [] });

        expect(screen.getByText('IF: eth0')).toBeTruthy();
        expect(screen.getByText('PROTO: tcp')).toBeTruthy();
    });
});
