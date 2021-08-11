/** @license ObjBrowse
 * Copyright 2021 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

import React from "react";

import { ViewProps, ViewScroller } from "./objbrowse";
import { FetchJSON } from "./hooks";
import { Ranges } from "./ranges";

import "./hexview.css";

type json = { Addr: string, Data: string }

function HexViewer(props: ViewProps) {
    return (<FetchJSON url={`/sym/${props.value.entity.id}/hex`}>
        {v =>
            <ViewScroller value={props.value}>
                <HexViewer1 {...props} v={v} />
            </ViewScroller>}
    </FetchJSON>);
}

function HexViewer1(props: ViewProps & { v: json }) {
    const v = props.v;

    // Format column headers.
    let hexHead = "", asciiHead = "";
    for (let i = 0; i < 16; i++) {
        if (i == 8) {
            hexHead += "  ";
        } else if (i > 0) {
            hexHead += " ";
        }
        const digit = i.toString(16)
        hexHead += " " + digit;
        asciiHead += digit;
    }

    // We always start the first row 16-byte aligned so it's easy to
    // visually combine the address with the header row. startOffset is
    // the offset of the top-left byte from v.Addr (it will be between
    // -15 and 0).
    const dataStart = BigInt("0x" + v.Addr);
    const startOffset = -Number(dataStart % BigInt(16));
    const len = v.Data.length / 2;

    // TODO: Support for selecting ranges, both in the hex or the ASCII.
    // Measure text dimensions by creating a hidden div with a
    // placeholder hex line with spans around the elements. From that we
    // can map the X coordinate (and if we move away from table rows,
    // use the height to map the Y coordinate). Or maybe I just register
    // a mouseUp handler and adjust the window.getSelection at that
    // point (check that that works with dragging, double clicking, and
    // shift-clicking).
    //
    // Ideally we would build our selection on the DOM Selection API to
    // stay consistent with platform selection rules, but modifying the
    // selection while selection is in progress aborts the selection
    // drag, and there doesn't appear to be a way to detect that the
    // user is done making a selection and then modify it. So we
    // implement our own mouse handling.

    // TODO: Jump to address.

    // TODO: Fetch data lazily and render only on scroll. For this I
    // probably want to break the data into large blocks and place those
    // in a CSS grid. The blocks can always exist, so I can attach
    // intersection observers to them and populate them as they come
    // into view. The grid would also make it easy to keep the selection
    // order as desired.

    const [addrs, offsets, hex, ascii] = format(v.Data, dataStart, startOffset, props.value.ranges);

    const addrWidth = 2 + (dataStart + BigInt(len)).toString(16).length;
    const offsetWidth = 3 + len.toString(16).length;
    return (
        <div>
            <table className="hv-table" >
                <colgroup>
                    <col style={{ width: addrWidth + "ch" }}></col>
                    <col style={{ width: offsetWidth + "ch" }}></col>
                    <col style={{ width: (3 * 16) + "ch" }}></col>
                    <col style={{ width: "16ch" }}></col>
                </colgroup>
                <thead><tr><th></th><th></th><th>{hexHead}</th><th>{asciiHead}</th></tr></thead>
                <tbody><tr>
                    <td className="hv-addr">{addrs}</td>
                    <td className="hv-addr">{offsets}</td>
                    <td>{hex}</td>
                    <td>{ascii}</td>
                </tr></tbody>
            </table>
        </div>);
}

/**
 * Format formats data as an address column, an offsets column, a hex
 * column, and an ASCII column.
 * @param dataStart is the address of data[0].
 * @param startOffset is the offset from dataStart at which the table
 * begins. This will be 0 or negative.
 * @param highlight is the set of ranges to select in the output.
 */
function format(data: string, dataStart: bigint, startOffset: number, highlight: Ranges) {
    const dataLen = data.length / 2;

    let addrs = "";
    let offsets = "";
    let line = "";
    let ascii = "";

    const ranges = highlight.ranges;
    let rangeIndex = 0;
    let highlighting = false;
    let lineHighlights: number[] = [], asciiHighlights: number[] = [];
    const pushHighlight = () => {
        lineHighlights.push(line.length);
        asciiHighlights.push(ascii.length);
        highlighting = !highlighting;
    }

    for (let i = 0; startOffset + i < dataLen; i++) {
        const dataOffset = startOffset + i; // Byte offset in data

        // End highlighting before we add spacing.
        if (highlighting && ranges[rangeIndex].end - dataStart <= dataOffset) {
            pushHighlight();
            rangeIndex++;
        }

        const col = i % 16;
        if (col == 8) {
            line += "  ";
        } else if (col == 0 && i != 0) {
            addrs += "\n";
            offsets += "\n";
            line += "\n";
            ascii += "\n";
        } else if (i > 0) {
            line += " ";
        }

        if (col == 0) {
            addrs += "0x" + (dataStart + BigInt(dataOffset)).toString(16);
            if (startOffset + i >= 0) {
                // Omit the offset on the first row if it would be negative.
                offsets += "+0x" + (startOffset + i).toString(16);
            }
        }

        if (dataOffset < 0) {
            // Before the beginning of the data.
            line += "  ";
            ascii += " ";
            continue;
        }

        if (!highlighting && ranges.length > rangeIndex && ranges[rangeIndex].start - dataStart <= dataOffset) {
            // Start highlighting.
            pushHighlight();
        }

        const hex = data.substr((startOffset + i) * 2, 2);
        line += hex;

        const val = parseInt(hex, 16);
        if (32 <= val && val <= 126)
            ascii += String.fromCharCode(val);
        else
            ascii += ".";
    }

    // Finally, go back and wrap ranges in highlights.
    if (highlighting) {
        pushHighlight();
    }
    const doHighlight = (text: string, highlights: number[]) => {
        if (highlights.length == 0) {
            return <>{text}</>;
        }
        let pos = 0;
        let out = [];
        for (let i = 0; i < highlights.length; i += 2) {
            out.push(text.substring(pos, highlights[i]));
            out.push(<span key={i} className="ob-selected">{text.substring(highlights[i], highlights[i + 1])}</span>)
            pos = highlights[i + 1];
        }
        out.push(text.substring(pos));
        return <>{out}</>;
    };

    return [addrs, offsets, doHighlight(line, lineHighlights), doHighlight(ascii, asciiHighlights)];
}

export const HexView = { element: HexViewer, id: "hex", label: "Hex" };
