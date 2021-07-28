/** @license ObjBrowse
 * Copyright 2021 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

import React from "react";
import ReactDOM from "react-dom";

import { App } from "./objbrowse";
import { SourceView } from "./sourceview";
import { AsmView } from "./asmview";
import { HexView } from "./hexview";

function main() {
    ReactDOM.render(
        <App views={[SourceView, AsmView, HexView]} />,
        document.getElementById('root')
    );
}

main();
