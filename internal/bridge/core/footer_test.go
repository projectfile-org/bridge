// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import "testing"

func TestGeneratedFooter(t *testing.T) {
	cases := []struct {
		name           string
		filename, lang string
		want           string
	}{{
		name: "readme english", filename: FileReadme, lang: "en",
		want: "*Generated from projectfile ([learn how](https://projectfile.org/how-to/readme))*",
	}, {
		name: "support spanish", filename: FileSupport, lang: "es",
		want: "*Generado desde projectfile ([saber cómo](https://projectfile.org/how-to/support))*",
	}, {
		name: "code of conduct ukrainian", filename: FileCodeOfConduct, lang: "uk",
		want: "*Згенеровано з projectfile ([дізнатися як](https://projectfile.org/how-to/code-of-conduct))*",
	}, {
		name: "unknown lang falls back to english", filename: FileSecurity, lang: "fr",
		want: "*Generated from projectfile ([learn how](https://projectfile.org/how-to/security))*",
	}, {
		name: "unknown filename falls back to readme slug", filename: "WEIRD.md", lang: "en",
		want: "*Generated from projectfile ([learn how](https://projectfile.org/how-to/readme))*",
	}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GeneratedFooter(tc.filename, tc.lang); got != tc.want {
				t.Errorf("GeneratedFooter(%q, %q) = %q, want %q", tc.filename, tc.lang, got, tc.want)
			}
		})
	}
}
