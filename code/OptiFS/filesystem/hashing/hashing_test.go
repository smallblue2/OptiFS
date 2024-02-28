package hashing

import (
	"syscall"
	"testing"

	"lukechampine.com/blake3"
)

// Helper function
func blake3Hash(in []byte) (out [64]byte) {
	return blake3.Sum512(in)
}

// Unit test for HashContents in hash.go
func TestHashContents(t *testing.T) {
	testcases := []struct {
		name           string
		input          []byte
		inputFlags     uint32
		expectedOutput [64]byte
	}{
		{
			name:           "Small byte slice - Append flag",
			input:          []byte("foobar"),
			inputFlags:     syscall.O_APPEND,
			expectedOutput: blake3Hash([]byte("foobar")),
		},
		{
			name:           "Medium byte slice",
			input:          []byte("Lorem ipsum dolor sit amet, consectetur adipiscing elit. Vivamus eget dui nec tellus pretium dapibus. Sed nunc risus, lacinia sed volutpat at, pretium sed metus. Morbi aliquam eu neque vitae tempor. Nullam bibendum tempus odio quis tincidunt. Mauris rutrum eros id porta pulvinar. Interdum et malesuada fames ac ante ipsum primis in faucibus. Etiam vel suscipit nisl. Quisque pellentesque, tortor eget pretium vehicula, nunc libero aliquam orci, a iaculis lectus nisl dapibus dui. Maecenas in accumsan libero."),
			inputFlags:     syscall.O_CREAT,
			expectedOutput: blake3Hash([]byte("Lorem ipsum dolor sit amet, consectetur adipiscing elit. Vivamus eget dui nec tellus pretium dapibus. Sed nunc risus, lacinia sed volutpat at, pretium sed metus. Morbi aliquam eu neque vitae tempor. Nullam bibendum tempus odio quis tincidunt. Mauris rutrum eros id porta pulvinar. Interdum et malesuada fames ac ante ipsum primis in faucibus. Etiam vel suscipit nisl. Quisque pellentesque, tortor eget pretium vehicula, nunc libero aliquam orci, a iaculis lectus nisl dapibus dui. Maecenas in accumsan libero.")),
		},
		{
			name:           "Large byte slice",
			input:          []byte("Nunc ligula mi, hendrerit ac tortor vestibulum, egestas bibendum nisl. Nullam dignissim felis ut mauris posuere viverra. Fusce aliquam molestie ex, nec facilisis risus tincidunt sit amet. Morbi ligula felis, vestibulum id dui at, porta tristique tellus. Cras justo ex, varius eget vehicula sed, ornare ut ex. Orci varius natoque penatibus et magnis dis parturient montes, nascetur ridiculus mus. Sed congue turpis et lacinia ullamcorper. Duis consequat ante lectus, in pretium justo porta non. Pellentesque laoreet, justo in vulputate condimentum, ligula lorem volutpat leo, vel laoreet neque dolor sit amet augue. Sed justo tortor, eleifend a dui semper, placerat auctor ipsum. Aenean commodo nisl ut turpis fringilla, ut commodo eros accumsan. Sed orci dolor, rutrum eget malesuada in, vestibulum sed eros. Pellentesque volutpat luctus maximus. In fermentum sodales lectus finibus egestas. Ut cursus nisi nec laoreet facilisis. Morbi non lacus ultrices nunc egestas finibus volutpat ac ipsum. Nam facilisis semper sodales. Vivamus ultrices augue eros, id placerat libero venenatis sit amet. Quisque lacinia, nibh in convallis viverra, sem diam sagittis est, id malesuada ante est id lorem. Nam pulvinar, velit sit amet dapibus cursus, sapien elit fermentum nunc, ut finibus leo quam at erat. Nulla at elit nulla. Mauris lorem tortor, sagittis id lectus non, pharetra maximus lectus. Morbi sem mauris, vulputate ac nisl vitae, fringilla gravida metus. Nulla semper tincidunt pretium. Nam diam nunc, blandit vel felis non, consequat ultricies erat. Nulla et arcu sollicitudin dolor accumsan consequat sed vitae arcu. In sit amet tristique tellus. Morbi mauris arcu, fringilla et venenatis sed, iaculis ut libero. Aenean cursus ante quis ligula eleifend, eu pulvinar dolor malesuada. Fusce et enim id dui auctor placerat. Sed faucibus ullamcorper blandit. In non ipsum sit amet nisl eleifend iaculis. Aenean ullamcorper tortor in sem porta, quis dignissim leo luctus. Pellentesque habitant morbi tristique senectus et netus et malesuada fames ac turpis egestas. Nullam pharetra ipsum sed semper convallis. Vivamus ac dui egestas, dignissim tortor ut, pulvinar arcu. Quisque ipsum magna, rutrum sit amet facilisis in, malesuada vel urna. Phasellus tincidunt nisl ac diam facilisis egestas. Quisque auctor, nisl et porta tincidunt, erat quam cursus risus, eu ultrices ante turpis non elit. Vestibulum non volutpat nibh. Aenean tristique ultricies neque ut porta. Vestibulum varius vulputate dolor, sed sollicitudin ante lacinia vitae. Cras in auctor tellus. Ut sem tellus, tristique in viverra ac, elementum eu sapien. Maecenas eget velit vel nisl viverra luctus id in ligula. Pellentesque dapibus odio ac ornare ornare. Quisque eget est erat. Maecenas id diam egestas, placerat odio a, eleifend sapien. Fusce scelerisque, sapien quis consequat ultricies, purus ipsum fringilla dui, nec fermentum nisl eros at enim."),
			inputFlags:     syscall.O_RDWR,
			expectedOutput: blake3Hash([]byte("Nunc ligula mi, hendrerit ac tortor vestibulum, egestas bibendum nisl. Nullam dignissim felis ut mauris posuere viverra. Fusce aliquam molestie ex, nec facilisis risus tincidunt sit amet. Morbi ligula felis, vestibulum id dui at, porta tristique tellus. Cras justo ex, varius eget vehicula sed, ornare ut ex. Orci varius natoque penatibus et magnis dis parturient montes, nascetur ridiculus mus. Sed congue turpis et lacinia ullamcorper. Duis consequat ante lectus, in pretium justo porta non. Pellentesque laoreet, justo in vulputate condimentum, ligula lorem volutpat leo, vel laoreet neque dolor sit amet augue. Sed justo tortor, eleifend a dui semper, placerat auctor ipsum. Aenean commodo nisl ut turpis fringilla, ut commodo eros accumsan. Sed orci dolor, rutrum eget malesuada in, vestibulum sed eros. Pellentesque volutpat luctus maximus. In fermentum sodales lectus finibus egestas. Ut cursus nisi nec laoreet facilisis. Morbi non lacus ultrices nunc egestas finibus volutpat ac ipsum. Nam facilisis semper sodales. Vivamus ultrices augue eros, id placerat libero venenatis sit amet. Quisque lacinia, nibh in convallis viverra, sem diam sagittis est, id malesuada ante est id lorem. Nam pulvinar, velit sit amet dapibus cursus, sapien elit fermentum nunc, ut finibus leo quam at erat. Nulla at elit nulla. Mauris lorem tortor, sagittis id lectus non, pharetra maximus lectus. Morbi sem mauris, vulputate ac nisl vitae, fringilla gravida metus. Nulla semper tincidunt pretium. Nam diam nunc, blandit vel felis non, consequat ultricies erat. Nulla et arcu sollicitudin dolor accumsan consequat sed vitae arcu. In sit amet tristique tellus. Morbi mauris arcu, fringilla et venenatis sed, iaculis ut libero. Aenean cursus ante quis ligula eleifend, eu pulvinar dolor malesuada. Fusce et enim id dui auctor placerat. Sed faucibus ullamcorper blandit. In non ipsum sit amet nisl eleifend iaculis. Aenean ullamcorper tortor in sem porta, quis dignissim leo luctus. Pellentesque habitant morbi tristique senectus et netus et malesuada fames ac turpis egestas. Nullam pharetra ipsum sed semper convallis. Vivamus ac dui egestas, dignissim tortor ut, pulvinar arcu. Quisque ipsum magna, rutrum sit amet facilisis in, malesuada vel urna. Phasellus tincidunt nisl ac diam facilisis egestas. Quisque auctor, nisl et porta tincidunt, erat quam cursus risus, eu ultrices ante turpis non elit. Vestibulum non volutpat nibh. Aenean tristique ultricies neque ut porta. Vestibulum varius vulputate dolor, sed sollicitudin ante lacinia vitae. Cras in auctor tellus. Ut sem tellus, tristique in viverra ac, elementum eu sapien. Maecenas eget velit vel nisl viverra luctus id in ligula. Pellentesque dapibus odio ac ornare ornare. Quisque eget est erat. Maecenas id diam egestas, placerat odio a, eleifend sapien. Fusce scelerisque, sapien quis consequat ultricies, purus ipsum fringilla dui, nec fermentum nisl eros at enim.")),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			returnValue := HashContents(tc.input, tc.inputFlags)

			if returnValue != tc.expectedOutput {
				t.Errorf("Expected %v, got %v\n.", tc.expectedOutput, returnValue)
			}
		})
	}
}
