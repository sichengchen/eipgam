package update

import (
	"testing"

	"github.com/sichengchen/eipgam/internal/testenv"
)

func TestMain(m *testing.M) {
	attestAPI = "http://127.0.0.1:1"
	githubAPI = "http://127.0.0.1:1"
	testenv.Main(m)
}
