/** @license ObjBrowse
 * Copyright 2021 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

import React from "react";

import { ViewProps } from "./objbrowse";
import { useFetchJSON } from "./hooks";

import "./hexview.css";

type json = { Addr: string, Data: string }

function HexViewer(props: ViewProps) {
    const fetch = useFetchJSON(`/sym/${props.entity.id}/hex`)
    if (fetch.pending) {
        return fetch.pending;
    }
    if (typeof fetch.value === "string") {
        // "No data" Error
        return <div>{fetch.value}</div>;
    }
    const v: json = fetch.value;

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

    // TODO: Doing this with long divs would create far fewer DOM elements. If I
    // layout each column in its own inline-block, selection would also be more
    // natural (though maybe I have to totally override that anyway). OTOH, if
    // I'm going to the trouble of making this lazy, a table may just be easier
    // and the cost irrelevant. CSS grid would work well for this.

    const rows: React.ReactElement[] = [];
    for (let off = startOffset; off < len; off += 16) {
        const [hex, ascii] = formatLine(v.Data, off);
        rows.push(
            <tr key={off}>
                <td>0x{(dataStart + BigInt(off)).toString(16)}</td>
                <td>{hex}</td>
                <td>{ascii}</td>
            </tr>
        )
    }

    const addrWidth = 2 + (dataStart + BigInt(len)).toString(16).length;
    return (<table className="hv-table" >
        <colgroup>
            <col style={{ width: addrWidth + "ch" }}></col>
            <col style={{ width: (3 * 16) + "ch" }}></col>
            <col style={{ width: "16ch" }}></col>
        </colgroup>
        <thead><tr><th></th><th>{hexHead}</th><th>{asciiHead}</th></tr></thead>
        <tbody>{rows}</tbody>
    </table>);
}

function formatLine(data: string, start: number) {
    const dataLen = data.length / 2;
    let line = "";
    let ascii = "";
    for (let i = 0; i < 16 && start + i < dataLen; i++) {
        if (i == 8) {
            line += "  ";
        } else if (i > 0) {
            line += " ";
        }
        if (start + i < 0) {
            // Before the beginning of the data.
            line += "  ";
            ascii += " ";
            continue;
        }

        const hex = data.substr((start + i) * 2, 2);
        line += hex;
        const val = parseInt(hex, 16);
        if (32 <= val && val <= 126)
            ascii += String.fromCharCode(val);
        else
            ascii += ".";
    }
    return [line, ascii];
}

export const HexView = { element: HexViewer, id: "hex", label: "Hex" };
