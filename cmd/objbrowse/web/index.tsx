/** @license ObjBrowse
 * Copyright 2021 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

import React from "react";
import ReactDOM from "react-dom";

import { App } from "./objbrowse";
import { HexView } from "./hexview";
import { AsmView } from "./asmview";

function main() {
    ReactDOM.render(
        <App views={[AsmView, HexView]} />,
        document.getElementById('root')
    );
}

main();
